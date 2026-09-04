package app

import "aiops-mvp/internal/safetyeval"

func (s *Service) SafetyEvaluation() (safetyeval.Report, error) {
	return safetyeval.RunDefault(s.Executor)
}
