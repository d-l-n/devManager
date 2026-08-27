package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBacklogItemSerialization(t *testing.T) {
	// Test that BacklogItem can be serialized and deserialized correctly
	now := time.Now()
	item := BacklogItem{
		ID:          "test-id",
		Title:       "Test Item",
		Description: "Test Description",
		Status:      "todo",
		Priority:    "high",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Serialize to JSON
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal backlog item: %v", err)
	}

	// Deserialize from JSON
	var item2 BacklogItem
	if err := json.Unmarshal(data, &item2); err != nil {
		t.Fatalf("unmarshal backlog item: %v", err)
	}

	// Compare fields
	if item.ID != item2.ID {
		t.Errorf("ID mismatch: %s vs %s", item.ID, item2.ID)
	}
	if item.Title != item2.Title {
		t.Errorf("Title mismatch: %s vs %s", item.Title, item2.Title)
	}
	if item.Description != item2.Description {
		t.Errorf("Description mismatch: %s vs %s", item.Description, item2.Description)
	}
	if item.Status != item2.Status {
		t.Errorf("Status mismatch: %s vs %s", item.Status, item2.Status)
	}
	if item.Priority != item2.Priority {
		t.Errorf("Priority mismatch: %s vs %s", item.Priority, item2.Priority)
	}
}

func TestProjectWithBacklog(t *testing.T) {
	// Test that Project with backlog items can be serialized and deserialized
	project := Project{
		Name: "Test Project",
		Path: "/test/path",
		Server: ServerConfig{
			Enabled:        true,
			Command:        "npm run dev",
			Port:           3000,
			URL:            "http://localhost:3000",
			StartupTimeout: 15000,
		},
		Playwright: PlaywrightConfig{
			Enabled:       true,
			Command:       "npx playwright test",
			UICommand:     "npx playwright test --ui",
			DebugCommand:  "npx playwright test --debug",
			ReportCommand: "npx playwright show-report",
		},
		Pinned: false,
		Backlog: []BacklogItem{
			{
				ID:          "item-1",
				Title:       "First Item",
				Description: "First description",
				Status:      "todo",
				Priority:    "high",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			{
				ID:          "item-2",
				Title:       "Second Item",
				Description: "Second description",
				Status:      "in-progress",
				Priority:    "medium",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
		},
	}

	// Serialize to JSON
	data, err := json.Marshal(project)
	if err != nil {
		t.Fatalf("marshal project: %v", err)
	}

	// Deserialize using ParseProject to test the full round-trip
	project2, err := ParseProject(data)
	if err != nil {
		t.Fatalf("parse project: %v", err)
	}

	// Compare basic fields
	if project.Name != project2.Name {
		t.Errorf("Name mismatch: %s vs %s", project.Name, project2.Name)
	}
	if project.Path != project2.Path {
		t.Errorf("Path mismatch: %s vs %s", project.Path, project2.Path)
	}

	// Compare backlog
	if len(project.Backlog) != len(project2.Backlog) {
		t.Fatalf("backlog length mismatch: %d vs %d", len(project.Backlog), len(project2.Backlog))
	}

	for i := range project.Backlog {
		if project.Backlog[i].ID != project2.Backlog[i].ID {
			t.Errorf("backlog item %d ID mismatch: %s vs %s", i, project.Backlog[i].ID, project2.Backlog[i].ID)
		}
		if project.Backlog[i].Title != project2.Backlog[i].Title {
			t.Errorf("backlog item %d Title mismatch: %s vs %s", i, project.Backlog[i].Title, project2.Backlog[i].Title)
		}
		if project.Backlog[i].Status != project2.Backlog[i].Status {
			t.Errorf("backlog item %d Status mismatch: %s vs %s", i, project.Backlog[i].Status, project2.Backlog[i].Status)
		}
		if project.Backlog[i].Priority != project2.Backlog[i].Priority {
			t.Errorf("backlog item %d Priority mismatch: %s vs %s", i, project.Backlog[i].Priority, project2.Backlog[i].Priority)
		}
	}
}