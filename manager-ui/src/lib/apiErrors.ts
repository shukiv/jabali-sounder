// Server-sent error text is translated at the display boundary: the API returns
// stable identifiers/messages (see manager-api handlers), and this maps them onto
// catalog keys so translators localise them in the same Weblate component as the
// rest of the UI. Unknown values fall through unchanged, so a new server error is
// still shown verbatim rather than swallowed.
import i18n from "../i18n";

/** Raw server string -> `errors.*` catalog key. */
const API_ERROR_KEYS: Record<string, string> = {
  "cannot delete the last owner": "cannot_delete_the_last_owner",
  "cannot delete yourself": "cannot_delete_yourself",
  "cannot demote the last owner": "cannot_demote_the_last_owner",
  "channel not found": "channel_not_found",
  "channels unavailable": "channels_unavailable",
  "current_password_incorrect": "current_password_incorrect",
  "decrypt_failed": "decrypt_failed",
  "duplicate name or token_id": "duplicate_name_or_token_id",
  "ends_at must be after starts_at": "ends_at_must_be_after_starts_at",
  "forbidden": "forbidden",
  "insufficient_token_scope": "insufficient_token_scope",
  "invalid ip/cidr: ": "invalid_ip_cidr",
  "invalid or expired token": "invalid_or_expired_token",
  "invalid panel hostname": "invalid_panel_hostname",
  "invalid role": "invalid_role",
  "invalid scope_type": "invalid_scope_type",
  "invalid severity": "invalid_severity",
  "invalid_code": "invalid_code",
  "invalid_credentials": "invalid_credentials",
  "invalid_session": "invalid_session",
  "invalid_tags": "invalid_tags",
  "maintenance unavailable": "maintenance_unavailable",
  "malformed_json": "malformed_json",
  "minutes must be 1..10080": "minutes_must_be_1_10080",
  "missing or invalid authorization header": "missing_or_invalid_authorization_header",
  "mute unavailable": "mute_unavailable",
  "name must be 1-200 chars": "name_must_be_1_200_chars",
  "new password must be at least 8 characters": "new_password_must_be_at_least_8_characters",
  "no_pending_2fa": "no_pending_2fa",
  "not_found": "not_found",
  "operator role required to export token secrets": "operator_role_required_to_export_token_secrets",
  "password must be at least 8 characters": "password_must_be_at_least_8_characters",
  "probe_failed": "probe_failed",
  "rate_limit_per_min out of range": "rate_limit_per_min_out_of_range",
  "rate_limited": "rate_limited",
  "request_body_too_large": "request_body_too_large",
  "scope_denied": "scope_denied",
  "scope_value required for this scope": "scope_value_required_for_this_scope",
  "server auth is misconfigured": "server_auth_is_misconfigured",
  "server_id and kind required": "server_id_and_kind_required",
  "session revoked or expired": "session_revoked_or_expired",
  "setup_already_completed": "setup_already_completed",
  "stored token secret can't be decrypted here \u2014 edit the server and re-enter the token secret": "stored_token_secret_can_t_be_decrypted_here",
  "threshold and duration must be non-negative": "threshold_and_duration_must_be_non_negative",
  "too_many_attempts": "too_many_attempts",
  "unknown metric": "unknown_metric",
  "unknown scope": "unknown_scope",
  "unsupported_settings_export": "unsupported_settings_export",
  "username already exists": "username_already_exists",
  "username is required": "username_is_required",
  "username must be 1-100 chars": "username_must_be_1_100_chars",
};

/**
 * Translate one server-sent error/detail string. Values carrying a trailing
 * payload (e.g. "invalid ip/cidr: 10.0.0.0/8") keep the payload appended.
 */
export function translateApiError(raw?: string | null): string {
  if (!raw) return "";
  const exact = API_ERROR_KEYS[raw];
  if (exact) return i18n.t(`errors.${exact}`);
  // "<known prefix>: <payload>" — translate the prefix, keep the payload.
  const sep = raw.indexOf(": ");
  if (sep > 0) {
    const head = raw.slice(0, sep + 2);
    const key = API_ERROR_KEYS[head] ?? API_ERROR_KEYS[raw.slice(0, sep)];
    if (key) return i18n.t(`errors.${key}`) + raw.slice(sep + 1);
  }
  return raw;
}
