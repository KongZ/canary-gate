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

import "github.com/KongZ/canary-gate/service"

//go:generate mockgen -destination=../mocks/mock_slack_client.go -package=mocks -mock_names=Client=MockSlackClient github.com/grafana/flagger-k6-webhook/pkg/slack Client

type Client interface {
	SendMessages(text string, hookType service.HookType, meta map[string]string) (map[string]string, error)
	UpdateMessages(slackMessages map[string]string, text, context string) error
	AddFileToThreads(slackMessages map[string]string, fileName, content string) error
}
