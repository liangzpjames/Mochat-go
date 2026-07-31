package domain

import "errors"

var (
	ErrInvalidTenantID           = errors.New("invalid tenant ID")
	ErrInvalidBusinessKey        = errors.New("invalid business key")
	ErrInvalidLeadName           = errors.New("invalid lead name")
	ErrUnsupportedLeadSource     = errors.New("unsupported lead source")
	ErrInvalidAssignmentStatus   = errors.New("invalid assignment status")
	ErrAssignmentVersionConflict = errors.New("assignment version conflict")
)
