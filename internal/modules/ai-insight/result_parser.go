package aiinsight

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	insightSchemaVersionV1 = 1
	insightSchemaVersionV2 = 2
)

func ParseSessionAnalysisResult(raw string, allowedMessageIDs map[string]struct{}) (SessionAnalysisResult, error) {
	var result SessionAnalysisResult
	if err := decodeInsightJSON(raw, &result); err != nil {
		return result, err
	}
	if result.SchemaVersion != insightSchemaVersionV1 && result.SchemaVersion != insightSchemaVersionV2 {
		return result, errors.New("schemaVersion must be 1 or 2")
	}
	if err := validateSessionVersionShape(raw, result.SchemaVersion); err != nil {
		return result, err
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
	if result.SchemaVersion == insightSchemaVersionV2 {
		if err := validateNullableScore("customer.qualityScore", result.Customer.QualityScore); err != nil {
			return result, err
		}
		for name, dimensions := range map[string][]QuantifiedDimension{
			"purchaseIntent.dimensions": result.Customer.PurchaseIntent.Dimensions,
			"churnRisk.dimensions":      result.Customer.ChurnRisk.Dimensions,
		} {
			if err := validateQuantifiedDimensions(name, dimensions, allowedMessageIDs); err != nil {
				return result, err
			}
		}
		for name, issues := range map[string][]UnresolvedIssue{
			"employeeQa.unresolvedCustomerIssues": result.EmployeeQA.UnresolvedCustomerIssues,
			"employeeQa.unresolvedObjections":     result.EmployeeQA.UnresolvedObjections,
		} {
			if err := validateUnresolvedIssues(name, issues, allowedMessageIDs); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func ParseSmartAnalysisResult(raw string, allowedMessageIDs map[string]struct{}) (SmartAnalysisResult, error) {
	var result SmartAnalysisResult
	if err := decodeInsightJSON(raw, &result); err != nil {
		return result, err
	}
	if result.SchemaVersion != insightSchemaVersionV1 && result.SchemaVersion != insightSchemaVersionV2 {
		return result, errors.New("schemaVersion must be 1 or 2")
	}
	if result.SchemaVersion == insightSchemaVersionV1 {
		if err := rejectRawFields(raw, "schemaVersion 1", "matchScore", "confidenceScore", "evidenceCoverageScore", "priorityScore", "priorityLevel", "dimensions"); err != nil {
			return result, err
		}
	}
	if strings.TrimSpace(result.Conclusion) == "" {
		return result, errors.New("conclusion must not be empty")
	}
	if result.SchemaVersion == insightSchemaVersionV1 {
		if result.Confidence != nil && (*result.Confidence < 0 || *result.Confidence > 1) {
			return result, errors.New("confidence must be between 0 and 1")
		}
	} else {
		if err := validateSmartV2Shape(raw); err != nil {
			return result, err
		}
		for name, score := range map[string]*float64{
			"matchScore": result.MatchScore, "confidenceScore": result.ConfidenceScore,
			"evidenceCoverageScore": result.EvidenceCoverageScore, "priorityScore": result.PriorityScore,
		} {
			if err := validateNullableScore(name, score); err != nil {
				return result, err
			}
		}
		if err := validateLevel("priorityLevel", result.PriorityLevel, true); err != nil {
			return result, err
		}
		if err := validateQuantifiedDimensions("dimensions", result.Dimensions, allowedMessageIDs); err != nil {
			return result, err
		}
	}
	if err := validateEvidence("evidenceMessageIds", result.EvidenceMessageIDs, allowedMessageIDs); err != nil {
		return result, err
	}
	return result, nil
}

func decodeInsightJSON(raw string, target any) error {
	value, err := normalizedInsightJSON(raw)
	if err != nil {
		return err
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

func normalizedInsightJSON(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if strings.HasPrefix(value, "```") {
		if !strings.HasPrefix(value, "```json") || !strings.HasSuffix(value, "```") {
			return "", errors.New("AI result must be JSON or a single json code fence")
		}
		value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "```json"), "```"))
	}
	if value == "" {
		return "", errors.New("AI result is empty")
	}
	return value, nil
}

func validateSmartV2Shape(raw string) error {
	value, err := normalizedInsightJSON(raw)
	if err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(value), &fields); err != nil {
		return fmt.Errorf("invalid AI result JSON: %w", err)
	}
	for _, name := range []string{"matchScore", "confidenceScore", "evidenceCoverageScore", "priorityScore", "priorityLevel", "dimensions"} {
		if _, ok := fields[name]; !ok {
			return fmt.Errorf("%s is required for schemaVersion 2", name)
		}
	}
	if _, ok := fields["confidence"]; ok {
		return errors.New("confidence is only valid for schemaVersion 1")
	}
	if err := validateRawDimensions(fields["dimensions"], "dimensions"); err != nil {
		return err
	}
	return nil
}

func validateSessionVersionShape(raw string, version int) error {
	root, err := rawInsightObject(raw)
	if err != nil {
		return err
	}
	customer, err := nestedRawObject(root, "customer")
	if err != nil {
		return err
	}
	purchaseIntent, err := nestedRawObject(customer, "purchaseIntent")
	if err != nil {
		return err
	}
	churnRisk, err := nestedRawObject(customer, "churnRisk")
	if err != nil {
		return err
	}
	employeeQA, err := nestedRawObject(root, "employeeQa")
	if err != nil {
		return err
	}
	if version == insightSchemaVersionV1 {
		if _, ok := customer["qualityScore"]; ok {
			return errors.New("customer.qualityScore is only valid for schemaVersion 2")
		}
		for name, assessment := range map[string]map[string]json.RawMessage{"purchaseIntent": purchaseIntent, "churnRisk": churnRisk} {
			if _, ok := assessment["dimensions"]; ok {
				return fmt.Errorf("customer.%s.dimensions is only valid for schemaVersion 2", name)
			}
		}
		for _, name := range []string{"unresolvedCustomerIssues", "unresolvedObjections"} {
			if _, ok := employeeQA[name]; ok {
				return fmt.Errorf("employeeQa.%s is only valid for schemaVersion 2", name)
			}
		}
		return nil
	}
	if _, ok := customer["qualityScore"]; !ok {
		return errors.New("customer.qualityScore is required for schemaVersion 2")
	}
	for name, assessment := range map[string]map[string]json.RawMessage{"purchaseIntent": purchaseIntent, "churnRisk": churnRisk} {
		if dimensions, ok := assessment["dimensions"]; ok {
			if err := validateRawDimensions(dimensions, "customer."+name+".dimensions"); err != nil {
				return err
			}
		}
	}
	return nil
}

func rejectRawFields(raw, version string, names ...string) error {
	fields, err := rawInsightObject(raw)
	if err != nil {
		return err
	}
	for _, name := range names {
		if _, ok := fields[name]; ok {
			return fmt.Errorf("%s is not valid for %s", name, version)
		}
	}
	return nil
}

func rawInsightObject(raw string) (map[string]json.RawMessage, error) {
	value, err := normalizedInsightJSON(raw)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(value), &fields); err != nil {
		return nil, fmt.Errorf("invalid AI result JSON: %w", err)
	}
	return fields, nil
}

func nestedRawObject(parent map[string]json.RawMessage, name string) (map[string]json.RawMessage, error) {
	value, ok := parent[name]
	if !ok {
		return nil, fmt.Errorf("%s is required", name)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(value, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("%s must be an object", name)
	}
	return fields, nil
}

func validateRawDimensions(raw json.RawMessage, name string) error {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return fmt.Errorf("%s must be an array", name)
	}
	var dimensions []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &dimensions); err != nil {
		return fmt.Errorf("%s must be an array", name)
	}
	for index, dimension := range dimensions {
		for _, field := range []string{"weight", "score"} {
			value, ok := dimension[field]
			if !ok {
				return fmt.Errorf("%s[%d].%s is required", name, index, field)
			}
			if field == "weight" && strings.TrimSpace(string(value)) == "null" {
				return fmt.Errorf("%s[%d].weight must be a number", name, index)
			}
		}
	}
	return nil
}

func validateNullableScore(name string, score *float64) error {
	if score != nil && (*score < 0 || *score > 100) {
		return fmt.Errorf("%s must be between 0 and 100", name)
	}
	return nil
}

func validateQuantifiedDimensions(name string, dimensions []QuantifiedDimension, allowed map[string]struct{}) error {
	for index, dimension := range dimensions {
		prefix := fmt.Sprintf("%s[%d]", name, index)
		if strings.TrimSpace(dimension.Name) == "" {
			return fmt.Errorf("%s.name must not be empty", prefix)
		}
		if dimension.Weight < 0 || dimension.Weight > 1 {
			return fmt.Errorf("%s.weight must be between 0 and 1", prefix)
		}
		if err := validateNullableScore(prefix+".score", dimension.Score); err != nil {
			return err
		}
		if strings.TrimSpace(dimension.Reason) == "" {
			return fmt.Errorf("%s.reason must not be empty", prefix)
		}
		if err := validateEvidence(prefix+".evidenceMessageIds", dimension.EvidenceMessageIDs, allowed); err != nil {
			return err
		}
	}
	return nil
}

func validateUnresolvedIssues(name string, issues []UnresolvedIssue, allowed map[string]struct{}) error {
	for index, issue := range issues {
		prefix := fmt.Sprintf("%s[%d]", name, index)
		if strings.TrimSpace(issue.Title) == "" {
			return fmt.Errorf("%s.title must not be empty", prefix)
		}
		if strings.TrimSpace(issue.Reason) == "" {
			return fmt.Errorf("%s.reason must not be empty", prefix)
		}
		if err := validateEvidence(prefix+".evidenceMessageIds", issue.EvidenceMessageIDs, allowed); err != nil {
			return err
		}
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
