package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"aiops-mvp/internal/domain"
	"aiops-mvp/internal/tools"
)

type evalCase struct {
	ID        string `json:"id"`
	Product   string `json:"product"`
	Question  string `json:"question"`
	GoldCause string `json:"gold_cause"`
}

func main() {
	input := flag.String("input", "eval/dataset.json", "source evaluation dataset")
	output := flag.String("output", "eval/replay-dataset-v1.json", "generated replay dataset")
	flag.Parse()

	raw, err := os.ReadFile(*input)
	if err != nil {
		fail("read dataset", err)
	}
	var source []evalCase
	if err := json.Unmarshal(raw, &source); err != nil {
		fail("parse dataset", err)
	}
	alerts := tools.DemoAlertProvider{}
	logs := tools.DemoLogProvider{}
	assets := tools.DemoAssetProvider{}
	out := make([]domain.FaultCaseRequest, 0, len(source))
	for _, item := range source {
		pid := strings.TrimSpace(item.Product)
		a, err := alerts.Alerts(pid)
		if err != nil {
			fail("load alerts for "+item.ID, err)
		}
		l, err := logs.Search(pid, item.Question)
		if err != nil {
			fail("load logs for "+item.ID, err)
		}
		as, err := assets.Assets(pid)
		if err != nil {
			fail("load assets for "+item.ID, err)
		}
		out = append(out, domain.FaultCaseRequest{
			Name: item.ID + " " + item.Question, ProductID: pid, Question: item.Question,
			GoldCause: item.GoldCause, Source: "synthetic", Version: "eval-v1",
			Tags: []string{"evaluation", item.ID, pid}, Alerts: a, Logs: l, Assets: as,
		})
	}
	encoded, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		fail("encode replay dataset", err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(*output, encoded, 0644); err != nil {
		fail("write replay dataset", err)
	}
	fmt.Printf("generated %d replay cases: %s\n", len(out), *output)
}

func fail(action string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", action, err)
	os.Exit(1)
}
