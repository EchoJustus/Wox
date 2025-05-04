package system

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"wox/common"
	"wox/plugin"
	"wox/util"
	"wox/util/shell"

	"github.com/samber/lo"
)

var qkIcon = plugin.PluginUrlIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &QkPlugin{})
}

type QkList struct {
	Url   string
	Icon  common.WoxImage
	Title string
}

type QkPlugin struct {
	api        plugin.API
	reg        *regexp.Regexp
	recentUrls []QkList
}

func (r *QkPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "1af58721-6c97-4901-b291-2333af08d9c9",
		Name:          "Quicker",
		Author:        "Echo Justus",
		Website:       "https://github.com/EchoJustus/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Open the Web App from List",
		Icon:          qkIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"*",
		},
		Commands: []plugin.MetadataCommand{},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
	}
}

func (r *QkPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	r.api = initParams.API
	r.reg = r.getReg()
	r.recentUrls = r.loadRecentUrls(ctx)
}

func (r *QkPlugin) loadRecentUrls(ctx context.Context) []QkList {
	urlsJson := r.api.GetSetting(ctx, "recentUrls")
	if urlsJson == "" {
		return []QkList{}
	}

	var urls []QkList
	err := json.Unmarshal([]byte(urlsJson), &urls)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("load recent urls error: %s", err.Error()))
		return []QkList{}
	}

	return urls
}

func (r *QkPlugin) getReg() *regexp.Regexp {
	// based on https://gist.github.com/dperini/729294
	return regexp.MustCompile(`^(http:\/\/www\.|https:\/\/www\.|http:\/\/|https:\/\/)?[a-z0-9]+([\-\.]{1}[a-z0-9]+)*\.[a-z]{2,5}(:[0-9]{1,5})?(\/.*)?$`)
}

func (r *QkPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	if len(query.Search) >= 2 {
		existingQkList := lo.Filter(r.recentUrls, func(item QkList, index int) bool {
			return strings.Contains(strings.ToLower(item.Url), strings.ToLower(query.Search))
		})

		for _, history := range existingQkList {
			results = append(results, plugin.QueryResult{
				Title:    history.Url,
				SubTitle: history.Title,
				Score:    100,
				Icon:     history.Icon.Overlay(qkIcon, 0.4, 0.6, 0.6),
				Actions: []plugin.QueryResultAction{
					{
						Name: "i18n:plugin_url_open",
						Icon: plugin.OpenIcon,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							openErr := shell.Open(history.Url)
							if openErr != nil {
								r.api.Log(ctx, "Error opening URL", openErr.Error())
							}
						},
					},
					{
						Name: "i18n:plugin_url_remove",
						Icon: plugin.TrashIcon,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							r.removeRecentUrl(ctx, history.Url)
						},
					},
				},
			})
		}
	}

	if len(r.reg.FindStringIndex(query.Search)) > 0 {
		results = append(results, plugin.QueryResult{
			Title:    query.Search,
			SubTitle: "i18n:plugin_url_open_in_browser",
			Score:    100,
			Icon:     qkIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name: "i18n:plugin_url_open",
					Icon: qkIcon,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						url := query.Search
						if !strings.HasPrefix(url, "http") {
							url = "https://" + url
						}
						openErr := shell.Open(url)
						if openErr != nil {
							r.api.Log(ctx, "Error opening URL", openErr.Error())
						} else {
							util.Go(ctx, "saveRecentUrl", func() {
								r.saveRecentUrl(ctx, url)
							})
						}
					},
				},
			},
		})
	}
	return
}

func (r *QkPlugin) saveRecentUrl(ctx context.Context, url string) {
	icon, err := getWebsiteIconWithCache(ctx, url)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("get url icon error: %s", err.Error()))
		icon = qkIcon
	}

	title := ""
	body, err := util.HttpGet(ctx, url)
	if err == nil {
		titleStart := strings.Index(string(body), "<title>")
		titleEnd := strings.Index(string(body), "</title>")
		if titleStart != -1 && titleEnd != -1 {
			title = string(body[titleStart+7 : titleEnd])
		}
	} else {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("get url title error: %s", err.Error()))
	}

	newHistory := QkList{
		Url:   url,
		Icon:  icon,
		Title: title,
	}

	// remove duplicate urls
	r.recentUrls = lo.Filter(r.recentUrls, func(item QkList, index int) bool {
		return item.Url != url
	})
	r.recentUrls = append([]QkList{newHistory}, r.recentUrls...)

	urlsJson, err := json.Marshal(r.recentUrls)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("save url setting error: %s", err.Error()))
		return
	}

	r.api.SaveSetting(ctx, "recentUrls", string(urlsJson), false)
}

func (r *QkPlugin) removeRecentUrl(ctx context.Context, url string) {
	r.recentUrls = lo.Filter(r.recentUrls, func(item QkList, index int) bool {
		return item.Url != url
	})

	urlsJson, err := json.Marshal(r.recentUrls)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("save url setting error: %s", err.Error()))
		return
	}

	r.api.SaveSetting(ctx, "recentUrls", string(urlsJson), false)
}
