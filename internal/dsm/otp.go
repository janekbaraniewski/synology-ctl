package dsm

import (
	"context"
	"errors"
	"fmt"
	"net/url"
)

// OTPRequiredError is returned by CallWithOTP when DSM rejects a call
// for lack of a (still-valid) one-time password. The TUI uses the
// typed error to pop a fresh-OTP modal and re-issue the action with
// the captured code.
//
// DSM signals OTP step-up via the same auth-area codes used at login
// time (403 = OTP required, 404 = OTP invalid/expired, 406 = OTP
// enforcement), even when the failing call is e.g. Share.Snapshot.create.
// We translate those into this typed error so callers don't have to
// hand-roll the IsOTPRequired check at every call site.
type OTPRequiredError struct {
	API    string
	Method string
	Code   int    // original DSM code (403 / 404 / 406)
	Reason string // short human-readable hint for the modal
}

func (e *OTPRequiredError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("dsm: %s.%s needs OTP step-up: %s", e.API, e.Method, e.Reason)
	}
	return fmt.Sprintf("dsm: %s.%s needs OTP step-up (code %d)", e.API, e.Method, e.Code)
}

// IsOTPStepupRequired reports whether err signals that the API call
// itself (not the login) needs an inline OTP code. The TUI uses this
// after every privileged write to decide between "show error" and
// "prompt for fresh OTP and retry".
func IsOTPStepupRequired(err error) bool {
	var stepup *OTPRequiredError
	return errors.As(err, &stepup)
}

// CallWithOTP performs a DSM API call with an inline one-time-password
// attached as `otp_code`. When DSM rejects the call because the OTP is
// missing, invalid, or expired (codes 403 / 404 / 406 — the same set
// IsOTPRequired recognises at login), the returned error is a typed
// *OTPRequiredError so callers can re-prompt without unwrapping
// generic *Error envelopes.
//
// Pass an empty otp on the first attempt: most snapshot-class APIs
// will then return the OTPRequiredError that the modal hooks onto.
// The second call with the captured code goes through.
func (c *Client) CallWithOTP(ctx context.Context, api string, version int, method string, params url.Values, otp string, out any) error {
	if params == nil {
		params = url.Values{}
	}
	if otp != "" {
		params.Set("otp_code", otp)
	}
	err := c.Call(ctx, api, version, method, params, out)
	if err == nil {
		return nil
	}
	// Wrap the auth-area codes — but only when this looks like an
	// OTP step-up, not a generic permission denial.
	if IsOTPRequired(err) {
		code := 0
		if e, ok := err.(*Error); ok {
			code = e.Code
		}
		reason := ""
		switch code {
		case 403:
			reason = "this call needs a fresh 2-step verification code"
		case 404:
			reason = "the 2-step verification code was invalid or has expired"
		case 406:
			reason = "DSM is enforcing 2-step verification for this call"
		}
		return &OTPRequiredError{API: api, Method: method, Code: code, Reason: reason}
	}
	return err
}
