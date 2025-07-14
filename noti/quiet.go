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
)

type QuietNoti struct {
}

// AddFileToThreads implements Client.
func (w QuietNoti) AddFileToThreads(slackMessages map[string]string, fileName string, content string) error {
	return nil
}

// SendMessages implements Client.
func (w QuietNoti) SendMessages(text string, hookType service.HookType, meta map[string]string) (map[string]string, error) {
	messages := map[string]string{}
	return messages, nil
}

// UpdateMessages implements Client.
func (w QuietNoti) UpdateMessages(slackMessages map[string]string, text string, context string) error {
	return nil
}

func NewQuietNoti() Client {
	return QuietNoti{}
}
