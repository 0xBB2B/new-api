package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
)

const (
	claudeResetProgramCedarEmber  = "cedar_ember"
	claudeResetProgramJuniperTide = "juniper_tide"
)

var claudeResetGrantIDPattern = regexp.MustCompile(`^[a-z0-9_-]{1,40}$`)

type ClaudeLimitResetRequest struct {
	Program    string `json:"program"`
	GrantID    string `json:"grant_id"`
	ResetsLeft int    `json:"resets_left"`
}

type ClaudeLimitResetResult struct {
	Success        bool
	MessageKey     string
	MessageArgs    map[string]any
	UpstreamStatus int
	Body           []byte
}

func validateClaudeLimitResetRequest(req ClaudeLimitResetRequest) string {
	switch req.Program {
	case claudeResetProgramJuniperTide:
		return ""
	case claudeResetProgramCedarEmber:
		if !claudeResetGrantIDPattern.MatchString(req.GrantID) || req.ResetsLeft < 1 || req.ResetsLeft > 100 {
			return i18n.MsgClaudeLimitResetInvalidParams
		}
		return ""
	default:
		return i18n.MsgClaudeLimitResetUnsupportedProgram
	}
}

func setClaudeResetHeaders(req *http.Request, accessToken, cliVersion string) {
	req.Header.Set("User-Agent", claudeCLIUserAgent(cliVersion))
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("Accept", "application/json")
}

func doClaudeResetRequest(client *http.Client, req *http.Request) (int, []byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

func fetchClaudeOAuthOrganizationUUID(ctx context.Context, client *http.Client, baseURL, accessToken, cliVersion string) (string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(strings.TrimSpace(baseURL), "/")+"/api/oauth/profile", nil)
	if err != nil {
		return "", 0, err
	}
	setClaudeResetHeaders(req, accessToken, cliVersion)
	status, body, err := doClaudeResetRequest(client, req)
	if err != nil || status < 200 || status >= 300 {
		return "", status, err
	}
	var profile struct {
		Organization struct {
			UUID string `json:"uuid"`
		} `json:"organization"`
	}
	if err := common.Unmarshal(body, &profile); err != nil {
		return "", status, err
	}
	orgUUID := profile.Organization.UUID
	// uuid.Parse 还接受 urn: / 花括号等形式，这里只放行标准 36 字符，防止拼进 URL 路径时夹带别的字符。
	if _, err := uuid.Parse(orgUUID); err != nil || len(orgUUID) != 36 {
		return "", status, errors.New("invalid organization uuid")
	}
	return orgUUID, status, nil
}

func postClaudeResetRateLimits(ctx context.Context, client *http.Client, baseURL, accessToken, cliVersion, orgUUID string, req ClaudeLimitResetRequest) (int, []byte, error) {
	payload := map[string]string{"program": req.Program}
	if req.Program == claudeResetProgramCedarEmber {
		payload["grant_id"] = req.GrantID
		// request_id 固定为 grant_id+剩余次数：重试同一次重置时上游按它去重，避免重复消耗。
		payload["request_id"] = fmt.Sprintf("%s-u%d", req.GrantID, req.ResetsLeft)
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return 0, nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(strings.TrimSpace(baseURL), "/")+"/api/organizations/"+orgUUID+"/reset_rate_limits", bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	setClaudeResetHeaders(httpReq, accessToken, cliVersion)
	httpReq.Header.Set("Content-Type", "application/json")
	return doClaudeResetRequest(client, httpReq)
}

func claudeLimitResetMessage(status int, body []byte) (bool, string, map[string]any) {
	if status < 200 || status >= 300 {
		switch status {
		case http.StatusTooManyRequests:
			return false, i18n.MsgClaudeLimitResetRateLimited, nil
		case http.StatusUnauthorized, http.StatusForbidden:
			return false, i18n.MsgClaudeLimitResetUnauthorized, nil
		default:
			return false, i18n.MsgClaudeLimitResetUpstreamStatus, map[string]any{"Status": status}
		}
	}
	var payload struct {
		Result string `json:"result"`
	}
	_ = common.Unmarshal(body, &payload)
	switch payload.Result {
	case "reset":
		return true, i18n.MsgClaudeLimitResetSuccess, nil
	case "already_used":
		return false, i18n.MsgClaudeLimitResetAlreadyUsed, nil
	case "not_limited":
		return false, i18n.MsgClaudeLimitResetNotLimited, nil
	case "cooldown":
		return false, i18n.MsgClaudeLimitResetCooldown, nil
	case "ineligible":
		return false, i18n.MsgClaudeLimitResetIneligible, nil
	case "unavailable":
		return false, i18n.MsgClaudeLimitResetUnavailable, nil
	default:
		return false, i18n.MsgClaudeLimitResetUnknownResult, nil
	}
}

func redeemClaudeLimitReset(ctx context.Context, client *http.Client, baseURL, accessToken, cliVersion string, req ClaudeLimitResetRequest) ClaudeLimitResetResult {
	if key := validateClaudeLimitResetRequest(req); key != "" {
		return ClaudeLimitResetResult{MessageKey: key}
	}
	if cliVersion == "" {
		return ClaudeLimitResetResult{MessageKey: i18n.MsgClaudeLimitResetNoClientVersion}
	}

	profileCtx, cancelProfile := context.WithTimeout(ctx, 15*time.Second)
	defer cancelProfile()
	orgUUID, profileStatus, err := fetchClaudeOAuthOrganizationUUID(profileCtx, client, baseURL, accessToken, cliVersion)
	if err != nil || orgUUID == "" {
		if err != nil {
			common.SysError("failed to fetch claude organization uuid: " + err.Error())
		}
		if profileStatus != 0 && (profileStatus < 200 || profileStatus >= 300) {
			_, key, args := claudeLimitResetMessage(profileStatus, nil)
			return ClaudeLimitResetResult{MessageKey: key, MessageArgs: args, UpstreamStatus: profileStatus}
		}
		return ClaudeLimitResetResult{MessageKey: i18n.MsgClaudeLimitResetOrgFailed, UpstreamStatus: profileStatus}
	}

	resetCtx, cancelReset := context.WithTimeout(ctx, 15*time.Second)
	defer cancelReset()
	status, body, err := postClaudeResetRateLimits(resetCtx, client, baseURL, accessToken, cliVersion, orgUUID, req)
	if err != nil {
		common.SysError("failed to reset claude rate limits: " + err.Error())
		return ClaudeLimitResetResult{MessageKey: i18n.MsgClaudeLimitResetResultUnknown, UpstreamStatus: status}
	}
	success, key, args := claudeLimitResetMessage(status, body)
	return ClaudeLimitResetResult{Success: success, MessageKey: key, MessageArgs: args, UpstreamStatus: status, Body: body}
}

func ResetClaudeChannelLimit(ctx context.Context, ch *model.Channel, cred *claudeOAuthCredential, req ClaudeLimitResetRequest) (ClaudeLimitResetResult, error) {
	if key := validateClaudeLimitResetRequest(req); key != "" {
		return ClaudeLimitResetResult{MessageKey: key}, nil
	}
	client, err := GetHttpClientWithProxy(ch.GetSetting().Proxy)
	if err != nil {
		return ClaudeLimitResetResult{}, err
	}

	versionCtx, cancelVersion := context.WithTimeout(ctx, 10*time.Second)
	cliVersion, versionErr := GetLatestClaudeCLIVersion(versionCtx, client)
	cancelVersion()
	if versionErr != nil {
		common.SysLog("failed to fetch latest claude cli version: " + versionErr.Error())
	}

	res := redeemClaudeLimitReset(ctx, client, ch.GetBaseURL(), cred.AccessToken, cliVersion, req)
	if (res.UpstreamStatus == http.StatusUnauthorized || res.UpstreamStatus == http.StatusForbidden) && strings.TrimSpace(cred.RefreshToken) != "" {
		if refreshed, _, refreshErr := RefreshClaudeChannelCredential(ctx, ch.Id, ClaudeCredentialRefreshOptions{ResetCaches: true}); refreshErr == nil {
			res = redeemClaudeLimitReset(ctx, client, ch.GetBaseURL(), refreshed.AccessToken, cliVersion, req)
		}
	}
	return res, nil
}
