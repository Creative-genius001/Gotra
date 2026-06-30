package ai

import "fmt"

// Prompt Engineering Layer (Backend Bible): each prompt is System Prompt +
// Context Block + Expected Output Schema, and all outputs must be structured
// JSON. Prompts are versioned by the constants below.
const promptVersion = "v1"

const analysisSchemaInstruction = `Respond with ONLY a single minified JSON object, no markdown, matching exactly:
{"explanation": string, "root_cause": string, "suggested_fix": string, "severity": "low"|"medium"|"high"|"critical", "confidence": integer 0-100}`

const incidentSchemaInstruction = `Respond with ONLY a single minified JSON object, no markdown, matching exactly:
{"summary": string, "timeline": string[], "affected_services": string[], "root_cause": string, "recommended_actions": string[], "severity": "low"|"medium"|"high"|"critical", "confidence": integer 0-100}`

const baseSystem = "You are Gotra AI, an expert backend debugging assistant embedded in a developer platform. " +
	"You analyze captured HTTP traffic, logs and metrics to find root causes. Be precise and concrete. " +
	"Set confidence based on how strongly the evidence supports your conclusion."

// systemFor returns the system prompt for an analysis type.
func systemFor(analysisType string) string {
	switch analysisType {
	case "generate_incident":
		return baseSystem + " " + incidentSchemaInstruction
	default:
		return baseSystem + " " + analysisSchemaInstruction
	}
}

// userForExplainError builds the user prompt for the Explain Error flow.
func userForExplainError(contextBlock string) string {
	return fmt.Sprintf("Explain why this request failed and how to fix it.\n\n%s", contextBlock)
}

// userForAnalyzeRequest builds the user prompt for general request analysis.
func userForAnalyzeRequest(contextBlock string) string {
	return fmt.Sprintf("Analyze this request/response. Identify problems, performance concerns and fixes.\n\n%s", contextBlock)
}

// userForAnalyzeReplay builds the user prompt for replay analysis.
func userForAnalyzeReplay(contextBlock string) string {
	return fmt.Sprintf("This request was replayed. Analyze the outcome, compare against expectations and suggest fixes.\n\n%s", contextBlock)
}

// userForExplainLogs builds the user prompt for the Explain Logs flow.
func userForExplainLogs(logs string) string {
	return fmt.Sprintf("Explain these application logs: summarize what happened, probable causes and suggested actions.\n\nLOGS:\n%s", truncate(logs))
}

// userForIncident builds the user prompt for incident generation.
func userForIncident(contextBlock string) string {
	return fmt.Sprintf("Generate an incident report from the following recent failed requests.\n\n%s", contextBlock)
}
