package generation

import "context"

type GenerateInput struct {
	CompanyName   string
	RoleTitle     string
	JDSignals     JDSignals
	Identity      string
	Contributions string
	Projects      string
	Education     string
	Credentials   string
	PriorFeedback string
}

type GenerateOutput struct {
	ResumeJSON []byte
}

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

func (g *Generator) Generate(_ context.Context, _ GenerateInput) (GenerateOutput, error) {
	return GenerateOutput{}, nil
}
