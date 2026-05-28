package channels

// PipelineSlashDefinition is transport metadata for a kernel/plugin slash command.
type PipelineSlashDefinition struct {
	Name        string
	Description string
}

// PipelineSlashConfigurer receives slash definitions for transport menus and dispatch.
type PipelineSlashConfigurer interface {
	SetPipelineSlashCommands(defs []PipelineSlashDefinition)
}
