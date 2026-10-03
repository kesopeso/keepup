package routes

import (
	"time"

	"keepup/apps/api/internal/config"
)

// PositionValidationError identifies a sample rejected by GPS quality validation.
type PositionValidationError struct{ Code string }

func (e *PositionValidationError) Error() string { return e.Code }

func rejectPosition(code string) error { return &PositionValidationError{Code: code} }

func validatePositionSample(input PositionUpdateInput, now time.Time, policy config.PositionValidationConfig) (PositionUpdateInput, error) {
	normalized, err := normalizePositionUpdateInput(input)
	if err != nil {
		return PositionUpdateInput{}, err
	}
	if normalized.AccuracyM == nil {
		return PositionUpdateInput{}, rejectPosition("accuracy_required")
	}
	if *normalized.AccuracyM > policy.MaxAccuracyM {
		return PositionUpdateInput{}, rejectPosition("accuracy_too_low")
	}
	if normalized.ClientRecordedAt == nil {
		return PositionUpdateInput{}, rejectPosition("timestamp_required")
	}
	stamp := normalized.ClientRecordedAt.UTC().Truncate(time.Microsecond)
	normalized.ClientRecordedAt = &stamp
	if stamp.Before(now.Add(-policy.MaxAge)) {
		return PositionUpdateInput{}, rejectPosition("timestamp_too_old")
	}
	if stamp.After(now.Add(policy.MaxFutureSkew)) {
		return PositionUpdateInput{}, rejectPosition("timestamp_in_future")
	}
	return normalized, nil
}

func validatePositionHistory(candidate, previous time.Time, distance, previousAccuracy, candidateAccuracy, maxSpeed float64) error {
	if candidate.Equal(previous) {
		return rejectPosition("duplicate_timestamp")
	}
	if candidate.Before(previous) {
		return rejectPosition("out_of_order_timestamp")
	}
	adjusted := max(0, distance-previousAccuracy-candidateAccuracy)
	if adjusted/candidate.Sub(previous).Seconds() > maxSpeed {
		return rejectPosition("impossible_jump")
	}
	return nil
}
