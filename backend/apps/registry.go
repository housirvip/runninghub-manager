package apps

import "context"

// AppInput is the input passed to a custom app's Execute method.
type AppInput struct {
	NodeInfoList []NodeInfo // parsed from task's nodeInfoList JSON
	UploadDir    string     // path to uploads directory
	OutputDir    string     // task-specific output directory (pre-created)
	BaseURL      string     // platform base URL for building output URLs
}

// NodeInfo is used for task execution input parsing.
type NodeInfo struct {
	NodeID     string `json:"nodeId"`
	FieldName  string `json:"fieldName"`
	FieldValue string `json:"fieldValue"`
}

// NodeInfoSchema describes an input field for apiCallDemo responses.
type NodeInfoSchema struct {
	NodeID    string      `json:"nodeId"`
	FieldName string      `json:"fieldName"`
	FieldType string      `json:"fieldType"` // FILE_PATH, TEXT, INT, SELECT, etc.
	FieldValue string     `json:"fieldValue"` // default value
	FieldData interface{} `json:"fieldData"`  // nil or options list
}

// AppResult is the output of a custom app execution.
type AppResult struct {
	Files []FileResult `json:"files"`
}

// FileResult describes one output file.
type FileResult struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Size     int64  `json:"size,omitempty"`
}

// CustomApp is the interface that all self-built AI applications must implement.
type CustomApp interface {
	ID() string          // unique identifier, used as webappId
	Name() string        // display name
	NodeInfoList() []NodeInfoSchema // describes input fields for apiCallDemo
	Execute(ctx context.Context, input AppInput) (*AppResult, error)
}

var registry = map[string]CustomApp{}

// Register adds a custom app to the global registry.
func Register(app CustomApp) {
	registry[app.ID()] = app
}

// Get returns a registered custom app by ID.
func Get(appID string) (CustomApp, bool) {
	app, ok := registry[appID]
	return app, ok
}

// IsCustomApp checks if the given webappID is a registered custom app.
func IsCustomApp(webappID string) bool {
	_, ok := registry[webappID]
	return ok
}

// List returns all registered custom apps.
func List() []CustomApp {
	result := make([]CustomApp, 0, len(registry))
	for _, app := range registry {
		result = append(result, app)
	}
	return result
}
