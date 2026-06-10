package generation

import "context"

type ExtractInput struct {
	JobDescription string
}

type JDSignals struct {
	PrioritySkills   []string `json:"priority_skills"`
	Seniority        string   `json:"seniority"`
	DomainVocabulary []string `json:"domain_vocabulary"`
}

type Extractor struct{}

func NewExtractor() *Extractor {
	return &Extractor{}
}

func (e *Extractor) Extract(_ context.Context, _ ExtractInput) (JDSignals, error) {
	return JDSignals{}, nil
}
