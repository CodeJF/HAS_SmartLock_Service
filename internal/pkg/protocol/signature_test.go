package protocol

import "testing"

func TestBuildUserSignatureIncludesAccessTokenWhenPresent(t *testing.T) {
	withToken := BuildUserSignature("POST", "token-123", "1.0.0", "test-app", "86", "req-1", "1715155200", map[string]any{
		"username": "user@example.com",
		"type":     "password",
	}, "test-app-secret")
	withoutToken := BuildUserSignature("POST", "", "1.0.0", "test-app", "86", "req-1", "1715155200", map[string]any{
		"username": "user@example.com",
		"type":     "password",
	}, "test-app-secret")

	const wantWithToken = "OGZkMzQ3NDYxMmQxYTJjNDUwNmY2MDM2YmQ5YmU5M2IwZDZiN2Y1ZjE3NjQyOGY3YWIwN2UyYTNlNDBhMWEwZA=="
	const wantWithoutToken = "YzU2NDViODk5ZDA1ZjhjMDQ0ZjhkYTZlNjQ5MWZlZTMwYzdkOTYxNDRiY2ZkZjVlOWUxYjViMjIyMWJiNWI4NQ=="
	if withToken != wantWithToken {
		t.Fatalf("BuildUserSignature() with token = %q, want %q", withToken, wantWithToken)
	}
	if withoutToken != wantWithoutToken {
		t.Fatalf("BuildUserSignature() without token = %q, want %q", withoutToken, wantWithoutToken)
	}
	if withToken == withoutToken {
		t.Fatalf("BuildUserSignature() should differ when access_token differs")
	}
}

func TestBuildUserSignatureIncludesEmptyBodyValuesLikeLegacyScript(t *testing.T) {
	sign := BuildUserSignature("POST", "", "1.0.0", "test-app", "86", "req-2", "1715155200", map[string]any{
		"country":  "",
		"username": "user@example.com",
	}, "test-app-secret")

	const want = "ZTZjNjZiNzJiMjNkNDcyNzZmYjI5ZTIwNTJiY2FiNWQ2NGRlYWZmZWY3NjBlMGQ4MzQyNmZmZGUzZDJjMDY3OQ=="
	if sign != want {
		t.Fatalf("BuildUserSignature() with empty string = %q, want %q", sign, want)
	}
}

func TestBuildDeviceSignatureUsesLegacyEncoding(t *testing.T) {
	sign := BuildDeviceSignature("POST", "SL100", "device-req-id", "1715155200", "", "device-test-uuid", map[string]any{
		"uid":     "u_123",
		"mac":     "AA:BB:CC:DD:EE:FF",
		"zone":    "8.00",
		"version": "1.0.0",
	}, "device-model-secret")

	const want = "ZWVhNzE3NThkZGQ5MDQ5Nzg5ZTMwYjBjMTUyNzA2YzhhN2U0ZjIyNmZmN2IxNjJlMTZjMzA2OWNlOWRjZTM4Nw=="
	if sign != want {
		t.Fatalf("BuildDeviceSignature() = %q, want %q", sign, want)
	}
}

func TestBuildDeviceSignatureIncludesEmptyBodyValuesLikeLegacyScript(t *testing.T) {
	sign := BuildDeviceSignature("POST", "SL100", "req-1", "1715155200", "", "device-1", map[string]any{
		"mac":     "AA:BB",
		"remark":  "",
		"version": "1.0.0",
	}, "device-model-secret")

	const want = "MTU1Y2I5NzliNjM5YTIwOGQyZGM1NzA3YjZiOTg5YTU1MWE4YjUyOWUxYzgyMGE3OTVkMDIxZDQ0ZGI0YTQ3ZA=="
	if sign != want {
		t.Fatalf("BuildDeviceSignature() with empty string = %q, want %q", sign, want)
	}
}

func TestBuildDeviceSignatureEncodesQueryValuesLikeLegacyScript(t *testing.T) {
	sign := BuildDeviceSignature("GET", "SL100", "req-2", "1715155200", "", "device-2", map[string]any{
		"name": "front door",
		"tag":  "A&B",
	}, "device-model-secret")

	const want = "NTI5YTlhMTYzNTY1NmUxOTdmZGUwMTU3YzcwMDk2YjJiOWFmZTk3NTg4NzdjODU5ZmJhZjQ3MjAzOTI0YTU0Zg=="
	if sign != want {
		t.Fatalf("BuildDeviceSignature() query encoding = %q, want %q", sign, want)
	}
}

func TestBuildDeviceSignatureIncludesUIDWhenPresent(t *testing.T) {
	sign := BuildDeviceSignature("POST", "SL100", "abc123def4567890", "1715155200", "u_123", "device-2", map[string]any{
		"version": "1.0.0",
		"zone":    "8.00",
	}, "device-model-secret")

	const want = "YTQzNzcyNTU0NDIyOGY3ZDYwOWQ5Y2U1YmZkMWQwYmExMjBiNzNlYjA5MDI2MzhlOTZjMDM3M2E4NDI5N2I3Mg=="
	if sign != want {
		t.Fatalf("BuildDeviceSignature() with uid = %q, want %q", sign, want)
	}
}
