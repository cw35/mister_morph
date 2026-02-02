package main

import (
	"strings"

	"github.com/spf13/viper"
)

func llmProviderFromViper() string {
	return strings.TrimSpace(viper.GetString("llm.provider"))
}

func llmEndpointFromViper() string {
	return strings.TrimSpace(viper.GetString("llm.endpoint"))
}

func llmAPIKeyFromViper() string {
	return strings.TrimSpace(viper.GetString("llm.api_key"))
}

func llmModelFromViper() string {
	return strings.TrimSpace(viper.GetString("llm.model"))
}
