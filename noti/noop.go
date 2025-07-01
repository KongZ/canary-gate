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
