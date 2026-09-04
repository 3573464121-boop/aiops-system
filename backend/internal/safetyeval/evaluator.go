package safetyeval

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"aiops-mvp/internal/domain"
	"aiops-mvp/internal/executor"
)

//go:embed safety-dataset-v1.json
var defaultDataset []byte

type Case struct {
	ID              string `json:"id"`
	Action          string `json:"action"`
	Risk            string `json:"risk"`
	Status          string `json:"status"`
	ExpectedKind    string `json:"expected_kind"`
	ExpectedAllowed bool   `json:"expected_allowed"`
}

type Dataset struct {
	Version string `json:"version"`
	Cases   []Case `json:"cases"`
}

type CaseResult struct {
	Case
	ActualKind    string `json:"actual_kind"`
	ActualAllowed bool   `json:"actual_allowed"`
	BlockReason   string `json:"block_reason"`
	DecisionOK    bool   `json:"decision_ok"`
	KindOK        bool   `json:"kind_ok"`
}

type Report struct {
	DatasetVersion         string       `json:"dataset_version"`
	GeneratedAt            time.Time    `json:"generated_at"`
	Total                  int          `json:"total"`
	ExpectedAllowed        int          `json:"expected_allowed"`
	ExpectedBlocked        int          `json:"expected_blocked"`
	CorrectDecisions       int          `json:"correct_decisions"`
	CorrectClassifications int          `json:"correct_classifications"`
	TrueBlocked            int          `json:"true_blocked"`
	FalseAllowed           int          `json:"false_allowed"`
	FalseBlocked           int          `json:"false_blocked"`
	DecisionAccuracy       float64      `json:"decision_accuracy"`
	ClassificationAccuracy float64      `json:"classification_accuracy"`
	BlockRecall            float64      `json:"block_recall"`
	UnsafeEscapeRate       float64      `json:"unsafe_escape_rate"`
	Passed                 bool         `json:"passed"`
	Results                []CaseResult `json:"results"`
}

func LoadDefault() (Dataset, error) {
	var dataset Dataset
	if err := json.Unmarshal(defaultDataset, &dataset); err != nil {
		return Dataset{}, fmt.Errorf("decode safety dataset: %w", err)
	}
	if dataset.Version == "" || len(dataset.Cases) == 0 {
		return Dataset{}, fmt.Errorf("safety dataset is empty")
	}
	return dataset, nil
}

func Evaluate(simulator *executor.Simulator, dataset Dataset) Report {
	report := Report{
		DatasetVersion: dataset.Version,
		GeneratedAt:    time.Now(),
		Total:          len(dataset.Cases),
		Results:        make([]CaseResult, 0, len(dataset.Cases)),
	}
	for _, testCase := range dataset.Cases {
		plan := simulator.Preview(domain.Approval{
			ID: "EVAL-" + testCase.ID, ProductID: "evaluation",
			Action: testCase.Action, Risk: testCase.Risk, Status: testCase.Status,
		})
		result := CaseResult{
			Case: testCase, ActualKind: plan.Kind, ActualAllowed: plan.Allowed,
			BlockReason: plan.BlockReason,
			DecisionOK:  plan.Allowed == testCase.ExpectedAllowed,
			KindOK:      plan.Kind == testCase.ExpectedKind,
		}
		if testCase.ExpectedAllowed {
			report.ExpectedAllowed++
			if !plan.Allowed {
				report.FalseBlocked++
			}
		} else {
			report.ExpectedBlocked++
			if plan.Allowed {
				report.FalseAllowed++
			} else {
				report.TrueBlocked++
			}
		}
		if result.DecisionOK {
			report.CorrectDecisions++
		}
		if result.KindOK {
			report.CorrectClassifications++
		}
		report.Results = append(report.Results, result)
	}
	if report.Total > 0 {
		report.DecisionAccuracy = float64(report.CorrectDecisions) / float64(report.Total)
		report.ClassificationAccuracy = float64(report.CorrectClassifications) / float64(report.Total)
	}
	if report.ExpectedBlocked > 0 {
		report.BlockRecall = float64(report.TrueBlocked) / float64(report.ExpectedBlocked)
		report.UnsafeEscapeRate = float64(report.FalseAllowed) / float64(report.ExpectedBlocked)
	}
	report.Passed = report.DecisionAccuracy == 1 && report.ClassificationAccuracy == 1 && report.UnsafeEscapeRate == 0
	return report
}

func RunDefault(simulator *executor.Simulator) (Report, error) {
	dataset, err := LoadDefault()
	if err != nil {
		return Report{}, err
	}
	return Evaluate(simulator, dataset), nil
}
