package openaiprovider

import (
	"embed"
	"fmt"
)

type Task string

const (
	TaskProfileExtraction Task = "profile_extraction"
	TaskJDAnalysis        Task = "jd_analysis"
	TaskMatchSuggestion   Task = "match_suggestion"
	TaskResumeDraft       Task = "resume_draft"
	TaskResumeHTMLCompose Task = "resume_html_compose"
)

type TaskDefinition struct {
	Task          Task
	PromptVersion string
	Prompt        string
	SchemaName    string
	Schema        []byte
}

//go:embed prompts/*.txt schemas/*.json
var resources embed.FS

var taskFiles = map[Task]struct {
	prompt string
	schema string
}{
	TaskProfileExtraction: {
		prompt: "prompts/profile_extraction_v1.txt",
		schema: "schemas/profile_extraction_v1.json",
	},
	TaskJDAnalysis: {
		prompt: "prompts/jd_analysis_v1.txt",
		schema: "schemas/jd_analysis_v1.json",
	},
	TaskMatchSuggestion: {
		prompt: "prompts/match_suggestion_v1.txt",
		schema: "schemas/match_suggestion_v1.json",
	},
	TaskResumeDraft: {
		prompt: "prompts/resume_draft_v1.txt",
		schema: "schemas/resume_draft_v1.json",
	},
	TaskResumeHTMLCompose: {
		prompt: "prompts/resume_html_compose_v1.txt",
		schema: "schemas/resume_html_compose_v1.json",
	},
}

func Definition(task Task) (TaskDefinition, error) {
	files, exists := taskFiles[task]
	if !exists {
		return TaskDefinition{}, fmt.Errorf("unknown OpenAI task %q", task)
	}
	prompt, err := resources.ReadFile(files.prompt)
	if err != nil {
		return TaskDefinition{}, fmt.Errorf(
			"read OpenAI prompt %q: %w",
			task,
			err,
		)
	}
	schema, err := resources.ReadFile(files.schema)
	if err != nil {
		return TaskDefinition{}, fmt.Errorf(
			"read OpenAI schema %q: %w",
			task,
			err,
		)
	}
	return TaskDefinition{
		Task:          task,
		PromptVersion: "v1",
		Prompt:        string(prompt),
		SchemaName:    string(task) + "_v1",
		Schema:        schema,
	}, nil
}

func Tasks() []Task {
	return []Task{
		TaskProfileExtraction,
		TaskJDAnalysis,
		TaskMatchSuggestion,
		TaskResumeDraft,
		TaskResumeHTMLCompose,
	}
}
