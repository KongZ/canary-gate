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
	"github.com/KongZ/canary-gate/service"
	"github.com/rs/zerolog/log"
)

type noopClient struct{}

func (c *noopClient) SendMessages(text string, _ service.HookType, _ map[string]string) (map[string]string, error) {
	if len(text) > 0 {
		log.Debug().Msgf("Slack disabled. Would've sent the following message: %s", text)
	}
	return nil, nil
}

func (c *noopClient) UpdateMessages(slackMessages map[string]string, text, _ string) error {
	if len(slackMessages) > 0 {
		log.Debug().Msgf("Slack disabled. Would've updated messages to: %s", text)
	}
	return nil
}

func (c *noopClient) AddFileToThreads(slackMessages map[string]string, fileName, _ string) error {
	if len(slackMessages) > 0 {
		log.Debug().Msgf("Slack disabled. Would've uploaded file named: %s", fileName)
	}
	return nil
}
