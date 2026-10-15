package ui_test

import (
	"errors"
	"fmt"
	"runtime"
	"testing"
)

type conformanceStatus uint8

const (
	conformancePassed conformanceStatus = iota
	conformanceFailed
	// conformanceSkipped marks a check whose precondition the test
	// environment cannot provide, such as a permission the user must grant.
	conformanceSkipped
)

// skipError reports that a check cannot run in this environment.
type skipError struct {
	reason string
}

func (err skipError) Error() string {
	return err.reason
}

// skipCheck returns an error that records the current check as skipped.
func skipCheck(format string, args ...any) error {
	return skipError{reason: fmt.Sprintf(format, args...)}
}

// skipOnDarwin returns a skip error on macOS, where AppKit has no public API
// for the feature. It returns nil on other platforms.
func skipOnDarwin(reason string) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	return skipCheck("AppKit cannot do this: %s", reason)
}

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
			case conformanceSkipped:
				t.Skip(result.Detail)
			default:
				t.Errorf("invalid conformance status %d", result.Status)
			}
		})
	}
}

func (report conformanceReport) summary() string {
	return fmt.Sprintf("%d passed, %d failed, %d skipped",
		report.passed(), report.failed(), report.count(conformanceSkipped))
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
			if errors.As(err, new(skipError)) {
				result.Status = conformanceSkipped
			}
			result.Detail = err.Error()
		}
	}()
	r.results = append(r.results, result)
}

func (r *recorder) report() conformanceReport {
	return conformanceReport{Results: append([]conformanceResult(nil), r.results...)}
}
