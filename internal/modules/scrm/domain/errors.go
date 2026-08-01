package domain

import "errors"

var (
	ErrInvalidTenantID           = errors.New("invalid tenant ID")
	ErrInvalidCorpID             = errors.New("invalid corp ID")
	ErrInvalidBusinessKey        = errors.New("invalid business key")
	ErrInvalidLeadName           = errors.New("invalid lead name")
	ErrInvalidLeadPhone          = errors.New("invalid lead phone")
	ErrUnsupportedLeadSource     = errors.New("unsupported lead source")
	ErrInvalidLeadTransition     = errors.New("invalid lead transition")
	ErrInvalidAssignmentStatus   = errors.New("invalid assignment status")
	ErrAssignmentVersionConflict = errors.New("assignment version conflict")
)
