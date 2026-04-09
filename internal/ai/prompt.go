package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PromptStrategy defines the strategy for prompt generation
type PromptStrategy string

const (
	StrategyBasic      PromptStrategy = "basic"
	StrategyChainOfThought PromptStrategy = "chain_of_thought"
	StrategyFewShot    PromptStrategy = "few_shot"
	StrategyAdvancedCoT PromptStrategy = "advanced_cot" // 新增：改进的 Cot 策略
)

// PromptBuilder builds prompts for hate speech detection
type PromptBuilder struct {
	SystemPrompt string
	strategy    PromptStrategy
	showReason  bool
}

// NewPromptBuilder creates a new prompt builder
func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{
		SystemPrompt: buildSystemPrompt(),
		strategy:    StrategyBasic,
		showReason:  false,
	}
}

// WithStrategy sets the prompt strategy
func (pb *PromptBuilder) WithStrategy(strategy PromptStrategy) *PromptBuilder {
	pb.strategy = strategy
	return pb
}

// WithReasoning enables or disables detailed reasoning output
func (pb *PromptBuilder) WithReasoning(showReason bool) *PromptBuilder {
	pb.showReason = showReason
	return pb
}

// buildSystemPrompt builds the system prompt
func buildSystemPrompt() string {
	return `You are an expert content moderation system specializing in detecting hate speech, harassment, and harmful content.

Your task is to analyze the provided content (text and/or images) and determine if it contains hate speech or harmful content.

Categories of hate speech:
1. Racial/Ethnic discrimination - derogatory language or discrimination based on race, ethnicity, or national origin
2. Religious intolerance - hate speech targeting religious groups or beliefs
3. Gender/Sexual orientation - misogyny, homophobia, transphobia, or gender-based attacks
4. Disability discrimination - mocking or derogatory language about disabilities
5. Age-based discrimination - ageism or discrimination based on age

Detection criteria:
- Explicit hate speech with slurs, epithets, or dehumanizing language
- Implicit hate speech using coded language, dog whistles, or veiled discrimination
- Incitement to violence or discrimination against protected groups
- Harassment or targeted attacks
- Promotion of supremacist ideologies

Response format:
Provide your response as JSON:
{
  "is_hate_speech": boolean,
  "confidence": float64 (0.0-1.0),
  "categories": ["category1", "category2"],
  "explanation": "detailed explanation of why the content was or wasn't classified as hate speech",
  "reasoning": "step-by-step analysis",
  "concerning_elements": ["element1", "element2"],
  "hate_type": "identity_hate | individual_hate | non_hate"
}`
}

// BuildTextPrompt builds a prompt for text analysis
func (pb *PromptBuilder) BuildTextPrompt(content string) string {
	switch pb.strategy {
	case StrategyAdvancedCoT:
		return pb.buildAdvancedChainOfThoughtPrompt(content)
	case StrategyChainOfThought:
		return pb.buildBasicChainOfThoughtPrompt(content)
	case StrategyFewShot:
		return pb.buildFewShotPrompt(content)
	default:
		return pb.buildBasicPrompt(content)
	}
}

// buildBasicPrompt builds a basic prompt
func (pb *PromptBuilder) buildBasicPrompt(content string) string {
	return fmt.Sprintf(`%s

Content to analyze:
%s

Please analyze the above text content and provide your assessment in JSON format.`,
		pb.SystemPrompt,
		content,
	)
}

// buildAdvancedChainOfThoughtPrompt builds an improved chain of thought prompt
// 使用您提供的 Cot 提示词模板
func (pb *PromptBuilder) buildAdvancedChainOfThoughtPrompt(content string) string {
	identities := "1. Race/ethnicity\n2. Religion\n3. Gender\n4. Sexual orientation\n5. Disability\n6. Nationality\n7. Age"
	dterms := " (such as slurs, derogatory terms, or hate speech)"

	prompt := fmt.Sprintf(`%s

Consider the following comment, and carefully answer the questions in each step to conclude whether it is hate speech or not:

Comment: "%s"

You don't need to tell me the answer to every step, just return these three conclusions: 
'Identity Hate', 'Individual Hate' or 'Non-hate'. 

Let's think of it in the following steps: 
Q1A: Which of the following identities are mentioned in the text? 
%s
Q1B: Are there any individuals mentioned explicitly by their name? 
Q2: Are there any derogatory, humiliating, insulting, or disparaging words or phrases%s specifically mentioned in the text? (Note: Colloquially usage of words should not be considered) 
Q3A: If Q2's answer is 'Yes', are those words or phrases directed towards or targeting your selected identities? 
Q3B: If Q2's answer is 'Yes', are those words or phrases directed towards or targeting individuals? 
Q4A: If Q3A's answer is 'Yes', do those terms have the possibility of inciting online hate against selected identities? 
Q4B: If Q3B's answer is 'Yes', do those terms have the possibility of inciting online hate against individuals? 
Q5A: If Q4A's answer is 'Yes', comment can be concluded as identity hate speech. Tell me your final conclusion: 'Identity Hate' or 'Non-hate'. 
Q5B: If Q4B's answer is 'Yes', comment can be concluded as individual hate speech. Tell me your final conclusion: 'Individual Hate' or 'Non-hate'. 

Please analyze step by step and provide your final assessment in JSON format with these fields:
{
  "is_hate_speech": boolean (true if Identity Hate or Individual Hate),
  "confidence": float64 (0.0-1.0),
  "categories": ["category1", "category2"],
  "explanation": "detailed explanation",
  "reasoning": "your step-by-step reasoning process",
  "hate_type": "identity_hate | individual_hate | non_hate"
}`,
		pb.SystemPrompt,
		content,
		identities,
		dterms,
	)

	if pb.showReason {
		prompt += "\n\nPlease provide your reasoning process step by step before giving the final conclusion."
	}

	return prompt
}

// buildBasicChainOfThoughtPrompt builds a basic chain of thought prompt
func (pb *PromptBuilder) buildBasicChainOfThoughtPrompt(content string) string {
	return fmt.Sprintf(`%s

Content to analyze:
%s

Please follow this reasoning process:
1. Identify the main themes and topics in the content
2. Look for specific hate speech indicators or discriminatory language
3. Consider the context and intent behind the language used
4. Evaluate the severity and potential harm of the content
5. Check for coded language or subtle forms of discrimination
6. Provide your final classification with reasoning

For each step, provide your analysis before moving to the next step. At the end, provide your final JSON response.`,
		pb.SystemPrompt,
		content,
	)
}

// BuildImagePrompt builds a prompt for image analysis
func (pb *PromptBuilder) BuildImagePrompt(imageDesc string) string {
	return fmt.Sprintf(`%s

Image to analyze:
%s

Please analyze the above image description and provide your assessment in JSON format.`,
		pb.SystemPrompt,
		imageDesc,
	)
}

// BuildMultimodalPrompt builds a prompt for multimodal (text + image) analysis
func (pb *PromptBuilder) BuildMultimodalPrompt(content, imageDesc string) string {
	switch pb.strategy {
	case StrategyAdvancedCoT:
		return pb.buildAdvancedMultimodalPrompt(content, imageDesc)
	default:
		return fmt.Sprintf(`%s

Text content:
%s

Image description:
%s

Please analyze both the text and image together, considering their combined context, and provide your assessment in JSON format.`,
			pb.SystemPrompt,
			content,
			imageDesc,
		)
	}
}

// buildAdvancedMultimodalPrompt builds advanced multimodal prompt with Cot
func (pb *PromptBuilder) buildAdvancedMultimodalPrompt(content, imageDesc string) string {
	identities := "1. Race/ethnicity\n2. Religion\n3. Gender\n4. Sexual orientation\n5. Disability\n6. Nationality\n7. Age"
	dterms := " (such as slurs, derogatory terms, or hate speech)"

	return fmt.Sprintf(`%s

Analyze both the text and image below, considering their combined context:

Text content:
%s

Image description:
%s

Consider the following comment, and carefully answer the questions in each step to conclude whether it is hate speech or not:

Q1A: Which of the following identities are mentioned in the text or visible in the image? 
%s
Q1B: Are there any individuals mentioned explicitly by their name or visible in the image? 
Q2: Are there any derogatory, humiliating, insulting, or disparaging words, phrases, or visual elements%s present? (Note: Colloquially usage of words should not be considered) 
Q3A: If Q2's answer is 'Yes', are those elements directed towards or targeting your selected identities? 
Q3B: If Q2's answer is 'Yes', are those elements directed towards or targeting individuals? 
Q4A: If Q3A's answer is 'Yes', do those terms have the possibility of inciting online hate against selected identities? 
Q4B: If Q3B's answer is 'Yes', do those terms have the possibility of inciting online hate against individuals? 
Q5A: If Q4A's answer is 'Yes', comment can be concluded as identity hate speech. Tell me your final conclusion: 'Identity Hate' or 'Non-hate'. 
Q5B: If Q4B's answer is 'Yes', comment can be concluded as individual hate speech. Tell me your final conclusion: 'Individual Hate' or 'Non-hate'. 

Please analyze step by step and provide your final assessment in JSON format with these fields:
{
  "is_hate_speech": boolean,
  "confidence": float64 (0.0-1.0),
  "categories": ["category1", "category2"],
  "explanation": "detailed explanation considering both text and image",
  "reasoning": "step-by-step reasoning process",
  "hate_type": "identity_hate | individual_hate | non_hate"
}`,
		pb.SystemPrompt,
		content,
		imageDesc,
		identities,
		dterms,
	)
}

// buildFewShotPrompt builds a few-shot learning prompt with examples
func (pb *PromptBuilder) buildFewShotPrompt(content string) string {
	examples := `Examples:

Example 1 - Identity Hate:
Input: "All people of [ethnicity] are criminals and should be deported."
Analysis: This contains explicit racial discrimination, uses generalizing language about an ethnic group, and calls for punitive action against a protected identity group.
Output: {"is_hate_speech": true, "confidence": 0.95, "categories": ["Racial/Ethnic discrimination"], "hate_type": "identity_hate", "explanation": "Explicit racial hate speech with generalizations about an ethnic group."}

Example 2 - Individual Hate:
Input: "@john_doe is a [slur] and deserves to die!"
Analysis: This contains a slur and death threat directed at a specific individual.
Output: {"is_hate_speech": true, "confidence": 0.98, "categories": ["Harassment"], "hate_type": "individual_hate", "explanation": "Contains explicit slur and threat directed at an individual."}

Example 3 - Non-Hate:
Input: "I disagree with the political policies mentioned in the article."
Analysis: This expresses a legitimate political opinion without attacking any protected group or individual.
Output: {"is_hate_speech": false, "confidence": 0.92, "categories": [], "hate_type": "non_hate", "explanation": "This is a legitimate political opinion expressing disagreement, not hate speech."}

`

	prompt := fmt.Sprintf(`%s

%s

Now analyze this content:
%s

Provide your response in JSON format following the same format as the examples.`,
		pb.SystemPrompt,
		examples,
		content,
	)

	return prompt
}

// ParseAIResponse parses the AI response
func ParseAIResponse(response string) (*DetectionResponse, error) {
	// Try to extract JSON from the response
	startIdx := strings.Index(response, "{")
	endIdx := strings.LastIndex(response, "}")

	if startIdx == -1 || endIdx == -1 {
		return nil, fmt.Errorf("no valid JSON found in response")
	}

	jsonStr := response[startIdx : endIdx+1]

	var result struct {
		IsHateSpeech      bool     `json:"is_hate_speech"`
		Confidence        float64  `json:"confidence"`
		Categories        []string `json:"categories"`
		Explanation       string   `json:"explanation"`
		Reasoning         string   `json:"reasoning,omitempty"`
		ConcerningElements []string `json:"concerning_elements,omitempty"`
		HateType         string   `json:"hate_type,omitempty"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	detectionResp := &DetectionResponse{
		IsHateSpeech: result.IsHateSpeech,
		Confidence:   result.Confidence,
		Categories:   result.Categories,
		Explanation:  result.Explanation,
	}

	if result.Reasoning != "" {
		detectionResp.Explanation += "\n\nReasoning: " + result.Reasoning
	}

	if len(result.ConcerningElements) > 0 {
		detectionResp.Explanation += "\n\nConcerning Elements: " + strings.Join(result.ConcerningElements, ", ")
	}

	// Add hate type to categories
	if result.HateType != "" {
		if !contains(result.Categories, result.HateType) {
			result.Categories = append(result.Categories, result.HateType)
			detectionResp.Categories = result.Categories
		}
	}

	return detectionResp, nil
}

// contains checks if a string slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

