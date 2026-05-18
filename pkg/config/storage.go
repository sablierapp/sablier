package config

// Storage holds the state persistence configuration.
type Storage struct {
	// File is the path to a file where Sablier persists its session state across restarts.
	// Leave empty to run stateless (state is lost on restart).
	// Env: SABLIER_STORAGE_FILE
	// CLI: --storage.file
	// Default: "" (stateless)
	File string
}

func NewStorageConfig() Storage {
	return Storage{
		File: "",
	}
}
