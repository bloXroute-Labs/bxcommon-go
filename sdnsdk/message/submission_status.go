package message

// SubmissionStatusBulkUpdate represents bulk update of submission statuses
type SubmissionStatusBulkUpdate struct {
	Statuses []SubmissionStatus `json:"statuses"`
}

// SubmissionStatus represents a single submission status
type SubmissionStatus struct {
	AccountID        string `json:"account_id"`
	SubmissionStatus bool   `json:"submission_status"`
}
