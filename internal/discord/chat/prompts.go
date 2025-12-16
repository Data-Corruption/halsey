package chat

import (
	_ "embed"
)

//go:embed prompts/intent_classifier.txt
var PromptIntentClassifier string

//go:embed prompts/response_gen.txt
var PromptResponseGen string

//go:embed prompts/runtime.txt
var PromptRuntime string
