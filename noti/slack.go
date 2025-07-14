/*
Copyright 2025 The canary-gate authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package noti

import (
	"fmt"
	"maps"
	"slices"

	"github.com/KongZ/canary-gate/service"
	"github.com/slack-go/slack"
)

type SlackOption struct {
	Token   string
	Channel string
	Debug   bool
}

type slackClientWrapper struct {
	client  *slack.Client
	channel string
}

func NewSlackClient(option SlackOption) Client {
	if option.Token == "" {
		return &QuietNoti{}
	}

	return &slackClientWrapper{
		client:  slack.New(option.Token, slack.OptionDebug(option.Debug)),
		channel: option.Channel,
	}
}

func (w *slackClientWrapper) SendMessages(text string, hookType service.HookType, meta map[string]string) (map[string]string, error) {
	slackMessages := map[string]string{}
	channelID, ts, _, err := w.client.SendMessage(w.channel, messageBlocks(text, slackHeader(hookType), meta))
	if err != nil {
		return nil, fmt.Errorf("error sending message to %s: %w", w.channel, err)
	}
	slackMessages[channelID] = ts
	return slackMessages, nil
}

func (w *slackClientWrapper) UpdateMessages(slackMessages map[string]string, text, context string) error {
	// for channelID, ts := range slackMessages {
	// 	if _, _, _, err := w.client.UpdateMessage(channelID, ts, messageBlocks(text, context)); err != nil {
	// 		return fmt.Errorf("error updating message %s in channel %s: %w", ts, channelID, err)
	// 	}
	// }
	return nil
}

func (w *slackClientWrapper) AddFileToThreads(slackMessages map[string]string, fileName, content string) error {
	for channelID, ts := range slackMessages {
		fileParams := slack.UploadFileV2Parameters{
			Title:           fileName,
			Content:         content,
			Channel:         channelID,
			ThreadTimestamp: ts,
		}
		if _, err := w.client.UploadFileV2(fileParams); err != nil {
			return fmt.Errorf("error while uploading output to %s in slack channel %s: %w", ts, channelID, err)
		}
	}

	return nil
}

func messageBlocks(text string, header string, meta map[string]string) slack.MsgOption {
	fields := make([]*slack.TextBlockObject, len(meta))
	keys := slices.Sorted((maps.Keys(meta)))
	for c, k := range keys {
		fields[c] = slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*%s*\n%s", k, meta[k]), false, false)
	}
	// TODO this should be change to randome ID but we need to store the ID in storage
	action := fmt.Sprintf("%s:%s:%s", meta[service.MetaCluster], meta[service.MetaNamespace], meta[service.MetaName])
	blocks := []slack.Block{
		slack.NewHeaderBlock(slack.NewTextBlockObject(slack.PlainTextType, header, true, false)),
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.PlainTextType, text, true, false), fields, nil),
		slack.NewActionBlock("",
			slack.NewButtonBlockElement("approve", "approve:"+action,
				slack.NewTextBlockObject("plain_text", "Approve", false, false),
			).WithStyle(slack.StylePrimary),
			slack.NewButtonBlockElement("halt", "halt:"+action,
				slack.NewTextBlockObject("plain_text", "Halt", false, false),
			).WithStyle(slack.StyleDanger),
		),
	}

	return slack.MsgOptionBlocks(blocks...)
}

func slackHeader(hook service.HookType) string {
	header := "Event"
	switch hook {
	case service.HookConfirmPromotion:
		header = "Confirm Promotion"
	case service.HookConfirmTrafficIncrease:
		header = "Confirm Traffic Increase"
	case service.HookConfirmRollout:
		header = "Confirm Rollout"
	case service.HookPostRollout:
		header = "Post Rollout"
	case service.HookPreRollout:
		header = "Pre- ollout"
	case service.HookRollback:
		header = "Rollback"
	case service.HookRollout:
		header = "Rollout"
	}
	return header
}
