package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/d-l-n/devmanager/internal/models"
)

// TriggerCIBuild dispara un build CI real (POST, timeout 15s del client).
// target: github | gitlab | jenkins. Devuelve un resumen (nunca incluye el
// token). Testeable con httptest: GitHub acepta APIBase override,
// GitLab/Jenkins usan sus BaseURL.
func TriggerCIBuild(target, projectName string, cfg models.CIConfig, client *http.Client) (string, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "github":
		return triggerGitHub(projectName, cfg.GitHub, client)
	case "gitlab":
		return triggerGitLab(projectName, cfg.GitLab, client)
	case "jenkins":
		return triggerJenkins(projectName, cfg.Jenkins, client)
	default:
		return "", fmt.Errorf("unknown CI target %q (use github, gitlab or jenkins)", target)
	}
}

func doPost(client *http.Client, req *http.Request) (int, string, error) {
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4000))
	return resp.StatusCode, string(raw), nil
}

func withCITimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 15*time.Second)
}

// triggerGitHub usa workflow_dispatch si hay workflow_file, si no
// repository_dispatch. APIBase permite apuntar a un mock en tests.
func triggerGitHub(projectName string, gh models.GitHubCIConfig, client *http.Client) (string, error) {
	if strings.TrimSpace(gh.Owner) == "" || strings.TrimSpace(gh.Repo) == "" {
		return "", fmt.Errorf("github CI not configured (owner/repo required)")
	}
	base := strings.TrimSuffix(strings.TrimSpace(gh.APIBase), "/")
	if base == "" {
		base = "https://api.github.com"
	}
	token := ResolveTokenRef(gh.TokenRef)
	ctx, cancel := withCITimeout()
	defer cancel()
	var endpoint string
	var payload interface{}
	if strings.TrimSpace(gh.WorkflowFile) != "" {
		ref := strings.TrimSpace(gh.Ref)
		if ref == "" {
			ref = "main"
		}
		endpoint = fmt.Sprintf("%s/repos/%s/%s/actions/workflows/%s/dispatches",
			base, gh.Owner, gh.Repo, gh.WorkflowFile)
		payload = map[string]interface{}{"ref": ref}
	} else {
		endpoint = fmt.Sprintf("%s/repos/%s/%s/dispatches", base, gh.Owner, gh.Repo)
		payload = map[string]interface{}{
			"event_type":     "devmanager",
			"client_payload": map[string]interface{}{"project": projectName},
		}
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	code, _, err := doPost(client, req)
	if err != nil {
		return "", err
	}
	if code == 204 || code == 201 || code == 200 {
		return fmt.Sprintf("github dispatch accepted (%d) for %s/%s", code, gh.Owner, gh.Repo), nil
	}
	return "", fmt.Errorf("github dispatch returned %d", code)
}

// triggerGitLab dispara un pipeline vía trigger token (form fields).
func triggerGitLab(projectName string, gl models.GitLabCIConfig, client *http.Client) (string, error) {
	if strings.TrimSpace(gl.BaseURL) == "" || strings.TrimSpace(gl.ProjectID) == "" {
		return "", fmt.Errorf("gitlab CI not configured (base_url/project_id required)")
	}
	token := ResolveTokenRef(gl.TokenRef)
	if token == "" {
		return "", fmt.Errorf("gitlab CI needs a token (set token_ref to env:NAME)")
	}
	ref := strings.TrimSpace(gl.Ref)
	if ref == "" {
		ref = "main"
	}
	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/trigger/pipeline",
		strings.TrimSuffix(strings.TrimSpace(gl.BaseURL), "/"), url.PathEscape(gl.ProjectID))
	form := url.Values{"token": {token}, "ref": {ref}}
	ctx, cancel := withCITimeout()
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	code, _, err := doPost(client, req)
	if err != nil {
		return "", err
	}
	if code == 201 || code == 200 {
		return fmt.Sprintf("gitlab pipeline created (%d) for project %s ref %s", code, gl.ProjectID, ref), nil
	}
	return "", fmt.Errorf("gitlab pipeline returned %d", code)
}

// triggerJenkins usa buildWithParameters?token= (o /build si falla con 400,
// Jenkins responde 201/302 en éxito según versión + security).
func triggerJenkins(projectName string, jk models.JenkinsCIConfig, client *http.Client) (string, error) {
	if strings.TrimSpace(jk.BaseURL) == "" || strings.TrimSpace(jk.Job) == "" {
		return "", fmt.Errorf("jenkins CI not configured (base_url/job required)")
	}
	token := ResolveTokenRef(jk.TokenRef)
	base := strings.TrimSuffix(strings.TrimSpace(jk.BaseURL), "/")
	job := strings.Trim(jk.Job, "/")
	ctx, cancel := withCITimeout()
	defer cancel()
	endpoint := fmt.Sprintf("%s/job/%s/buildWithParameters?token=%s",
		base, job, url.QueryEscape(token))
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, nil)
	if err != nil {
		return "", err
	}
	// No propagar token a logs: solo job/base en mensajes.
	code, _, err := doPost(client, req)
	if err != nil {
		return "", err
	}
	if code == 201 || code == 200 || code == 302 {
		return fmt.Sprintf("jenkins build queued (%d) for job %s", code, jk.Job), nil
	}
	return "", fmt.Errorf("jenkins build returned %d for job %s", code, jk.Job)
}
