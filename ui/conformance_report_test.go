package ui_test

import (
	"fmt"
	"testing"
)

type conformanceStatus uint8

const (
	conformancePassed conformanceStatus = iota
	conformanceFailed
)

type conformanceResult struct {
	Name   string
	Status conformanceStatus
	Detail string
}

type conformanceReport struct {
	Results []conformanceResult
}

func (report conformanceReport) passed() int {
	return report.count(conformancePassed)
}

func (report conformanceReport) failed() int {
	return report.count(conformanceFailed)
}

func (report conformanceReport) assert(t *testing.T) {
	t.Helper()
	for _, result := range report.Results {
		result := result
		t.Run(result.Name, func(t *testing.T) {
			switch result.Status {
			case conformancePassed:
				return
			case conformanceFailed:
				t.Error(result.Detail)
			default:
				t.Errorf("invalid conformance status %d", result.Status)
			}
		})
	}
}

func (report conformanceReport) summary() string {
	return fmt.Sprintf("%d passed, %d failed", report.passed(), report.failed())
}

func (report conformanceReport) count(status conformanceStatus) int {
	count := 0
	for _, result := range report.Results {
		if result.Status == status {
			count++
		}
	}
	return count
}

type recorder struct {
	results []conformanceResult
}

func (r *recorder) check(name string, check func() error) {
	result := conformanceResult{Name: name, Status: conformancePassed}
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				result.Status = conformanceFailed
				result.Detail = fmt.Sprintf("panic: %v", recovered)
			}
		}()
		if err := check(); err != nil {
			result.Status = conformanceFailed
			result.Detail = err.Error()
		}
	}()
	r.results = append(r.results, result)
}

func (r *recorder) report() conformanceReport {
	return conformanceReport{Results: append([]conformanceResult(nil), r.results...)}
}
