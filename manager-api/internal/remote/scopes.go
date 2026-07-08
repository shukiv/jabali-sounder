package remote

// Allowed automation scopes on a managed jabali2 server.
// These match jabali2's models.AllowedAutomationScopes exactly.
const (
	ScopeReadAll          = "read:*"
	ScopeReadDomains      = "read:domains"
	ScopeReadUsers        = "read:users"
	ScopeReadApplications = "read:applications"
	ScopeReadMail         = "read:mail"
	ScopeReadStatus       = "read:status"
	ScopeReadMetrics      = "read:metrics"
)
