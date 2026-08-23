package a2ui

// ProtocolVersion specifies the A2UI protocol standard.
const ProtocolVersion = "v0.9"

// A2UIEnvelope represents the root event payload streamed to Gemini Enterprise.
type A2UIEnvelope struct {
	Version          string                   `json:"version"`
	CreateSurface    *CreateSurfacePayload    `json:"createSurface,omitempty"`
	UpdateComponents *UpdateComponentsPayload `json:"updateComponents,omitempty"`
	UpdateDataModel  *UpdateDataModelPayload  `json:"updateDataModel,omitempty"`
	DeleteSurface    *DeleteSurfacePayload    `json:"deleteSurface,omitempty"`
}

// CreateSurfacePayload initializes a new interactive surface canvas.
type CreateSurfacePayload struct {
	SurfaceID string `json:"surfaceId"`
	CatalogID string `json:"catalogId"`
	Theme     string `json:"theme,omitempty"`
}

// ComponentDef defines a declarative UI widget in the component tree.
type ComponentDef struct {
	ID        string                 `json:"id"`
	Component string                 `json:"component"`
	Props     map[string]any         `json:"props,omitempty"`
	Children  []ComponentDef         `json:"children,omitempty"`
	Events    map[string]ActionEvent `json:"events,omitempty"`
}

// ActionEvent binds a UI action (e.g. click, select) to an agent callback.
type ActionEvent struct {
	Action string         `json:"action"`
	Params map[string]any `json:"params,omitempty"`
}

// UpdateComponentsPayload delivers the hierarchical component graph.
type UpdateComponentsPayload struct {
	SurfaceID string       `json:"surfaceId"`
	Root      ComponentDef `json:"root"`
}

// UpdateDataModelPayload pushes reactive state bindings to the surface.
type UpdateDataModelPayload struct {
	SurfaceID string         `json:"surfaceId"`
	Data      map[string]any `json:"data"`
}

// DeleteSurfacePayload tears down a surface.
type DeleteSurfacePayload struct {
	SurfaceID string `json:"surfaceId"`
}
