package mcp_market

import "strings"

var providerPresets = []MarketProviderPreset{
	{
		ID:          ProviderGeneric,
		Name:        "Custom",
		AuthType:    AuthTypeNone,
		Description: "Manually fill in the catalog/detail API URL to adapt to any MCP marketplace.",
	},
	{
		ID:                ProviderModelScope,
		Name:              "ModelScope",
		CatalogURL:        "https://www.modelscope.cn/openapi/v1/mcp/servers",
		DetailURLTemplate: "https://www.modelscope.cn/openapi/v1/mcp/servers/{raw_id}",
		AuthType:          AuthTypeBearer,
		Description:       "Fixed Bearer Token authentication, only fetches activated services (/operational).",
	},
}

func ListProviderPresets() []MarketProviderPreset {
	out := make([]MarketProviderPreset, len(providerPresets))
	copy(out, providerPresets)
	return out
}

func NormalizeProviderID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return ProviderGeneric
	}
	for _, preset := range providerPresets {
		if id == preset.ID {
			return id
		}
	}
	return ProviderGeneric
}

func GetProviderPreset(id string) (MarketProviderPreset, bool) {
	id = NormalizeProviderID(id)
	for _, preset := range providerPresets {
		if preset.ID == id {
			return preset, true
		}
	}
	return MarketProviderPreset{}, false
}
