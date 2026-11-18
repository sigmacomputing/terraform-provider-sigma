package plugins

// plugin is the response shape for GET/POST/PATCH /v2/plugins[/{pluginId}].
// description and devUrl are nullable in the API; url defaults to an empty
// string and devUrl to "http://localhost:5173" server-side when omitted.
type plugin struct {
	PluginID    string  `json:"pluginId"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	URL         string  `json:"url"`
	DevURL      *string `json:"devUrl"`
	Type        string  `json:"type"`
}

// createPluginReq is the POST /v2/plugins body. Only name is required; the
// optional fields are omitted when unset so the server applies its defaults.
type createPluginReq struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	URL         *string `json:"url,omitempty"`
	DevURL      *string `json:"devUrl,omitempty"`
}

// patchPluginReq is the PATCH /v2/plugins/{pluginId} body. Every field is a
// pointer so only the attributes that actually changed are sent. url isn't
// patchable.
type patchPluginReq struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	DevURL      *string `json:"devUrl,omitempty"`
}
