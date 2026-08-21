package aiinsight

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const insightSchemaVersion = 1

func ParseSessionAnalysisResult(raw string, allowedMessageIDs map[string]struct{}) (SessionAnalysisResult, error) {
	var result SessionAnalysisResult
	if err := decodeInsightJSON(raw, &result); err != nil {
		return result, err
	}
	if result.SchemaVersion != insightSchemaVersion {
		return result, fmt.Errorf("schemaVersion must be %d", insightSchemaVersion)
	}
	if strings.TrimSpace(result.Summary) == "" {
		return result, errors.New("summary must not be empty")
	}
	if err := validateAssessment("purchaseIntent", result.Customer.PurchaseIntent, allowedMessageIDs); err != nil {
		return result, err
	}
	if err := validateAssessment("churnRisk", result.Customer.ChurnRisk, allowedMessageIDs); err != nil {
		return result, err
	}
	if err := validateLevel("qualityLevel", result.Customer.QualityLevel, true); err != nil {
		return result, err
	}
	if err := validateEmotion(result.Customer.Emotion, allowedMessageIDs); err != nil {
		return result, err
	}
	if result.EmployeeQA.Score < 0 || result.EmployeeQA.Score > 100 {
		return result, fmt.Errorf("employeeQa.score must be between 0 and 100")
	}
	for index, dimension := range result.EmployeeQA.Dimensions {
		if strings.TrimSpace(dimension.Name) == "" {
			return result, fmt.Errorf("employeeQa.dimensions[%d].name must not be empty", index)
		}
		if dimension.Score < 0 || dimension.Score > 100 {
			return result, fmt.Errorf("employeeQa.dimensions[%d].score must be between 0 and 100", index)
		}
	}
	return result, nil
}

func ParseSmartAnalysisResult(raw string, allowedMessageIDs map[string]struct{}) (SmartAnalysisResult, error) {
	var result SmartAnalysisResult
	if err := decodeInsightJSON(raw, &result); err != nil {
		return result, err
	}
	if result.SchemaVersion != insightSchemaVersion {
		return result, fmt.Errorf("schemaVersion must be %d", insightSchemaVersion)
	}
	if strings.TrimSpace(result.Conclusion) == "" {
		return result, errors.New("conclusion must not be empty")
	}
	if result.Confidence != nil && (*result.Confidence < 0 || *result.Confidence > 1) {
		return result, errors.New("confidence must be between 0 and 1")
	}
	if err := validateEvidence("evidenceMessageIds", result.EvidenceMessageIDs, allowedMessageIDs); err != nil {
		return result, err
	}
	return result, nil
}

func decodeInsightJSON(raw string, target any) error {
	value := strings.TrimSpace(raw)
	if strings.HasPrefix(value, "```") {
		if !strings.HasPrefix(value, "```json") || !strings.HasSuffix(value, "```") {
			return errors.New("AI result must be JSON or a single json code fence")
		}
		value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "```json"), "```"))
	}
	if value == "" {
		return errors.New("AI result is empty")
	}
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid AI result JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return errors.New("AI result must contain one JSON object")
	}
	return nil
}

func validateAssessment(name string, assessment EvidenceAssessment, allowed map[string]struct{}) error {
	if err := validateLevel(name+".level", assessment.Level, true); err != nil {
		return err
	}
	if assessment.Score != nil && (*assessment.Score < 0 || *assessment.Score > 100) {
		return fmt.Errorf("%s.score must be between 0 and 100", name)
	}
	if strings.TrimSpace(assessment.Reason) == "" {
		return fmt.Errorf("%s.reason must not be empty", name)
	}
	return validateEvidence(name+".evidenceMessageIds", assessment.EvidenceMessageIDs, allowed)
}

func validateLevel(name, value string, allowInsufficient bool) error {
	value = strings.ToLower(strings.TrimSpace(value))
	valid := map[string]struct{}{"low": {}, "medium": {}, "high": {}}
	if allowInsufficient {
		valid["insufficient"] = struct{}{}
	}
	if _, ok := valid[value]; !ok {
		return fmt.Errorf("%s has unknown value %q", name, value)
	}
	return nil
}

func validateEmotion(emotion CustomerEmotion, allowed map[string]struct{}) error {
	label := strings.ToLower(strings.TrimSpace(emotion.Label))
	switch label {
	case "positive", "neutral", "negative", "mixed", "unknown":
	default:
		return fmt.Errorf("emotion.label has unknown value %q", emotion.Label)
	}
	if strings.TrimSpace(emotion.Reason) == "" {
		return errors.New("emotion.reason must not be empty")
	}
	return validateEvidence("emotion.evidenceMessageIds", emotion.EvidenceMessageIDs, allowed)
}

func validateEvidence(name string, ids []string, allowed map[string]struct{}) error {
	for _, id := range ids {
		if _, ok := allowed[id]; !ok {
			return fmt.Errorf("%s contains evidence message %q outside source window", name, id)
		}
	}
	return nil
}
