// Copyright 2021-2022 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/vmware-tanzu/kubeapps/pkg/chart/models"
)

// tests the GET /live endpoint
func Test_GetLive(t *testing.T) {
	_, cleanup := setMockManager(t)
	defer cleanup()

	ts := httptest.NewServer(setupRoutes())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/live")
	assert.NoError(t, err, "should not return an error")
	defer res.Body.Close()
	assert.Equal(t, res.StatusCode, http.StatusOK, "http status code should match")
}

// tests the GET /ready endpoint
func Test_GetReady(t *testing.T) {
	_, cleanup := setMockManager(t)
	defer cleanup()

	ts := httptest.NewServer(setupRoutes())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/ready")
	assert.NoError(t, err, "should not return an error")
	defer res.Body.Close()
	assert.Equal(t, res.StatusCode, http.StatusOK, "http status code should match")
}

// tests the GET /{apiVersion}/clusters/default/namespaces/{namespace}/charts/categories endpoint
// particularly, it just tests that the endpoint is running the expected count query
func Test_GetChartCategories(t *testing.T) {
	ts := httptest.NewServer(setupRoutes())
	defer ts.Close()

	tests := []struct {
		name                    string
		expectedChartCategories []*models.ChartCategory
	}{
		{
			"no charts",
			[]*models.ChartCategory{},
		},
		{
			"two charts - same category",
			[]*models.ChartCategory{
				{Name: "cat1", Count: 2},
			},
		},
		{
			"two charts - different category",
			[]*models.ChartCategory{
				{Name: "cat1", Count: 1},
				{Name: "cat2", Count: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setMockManager(t)
			defer cleanup()

			rows := sqlmock.NewRows([]string{"name", "count"})
			for _, chartCategories := range tt.expectedChartCategories {
				rows.AddRow(chartCategories.Name, chartCategories.Count)
			}
			mock.ExpectQuery("SELECT (info ->> 'category')*").
				WithArgs("my-namespace", globalReposNamespace).
				WillReturnRows(rows)

			res, err := http.Get(ts.URL + pathPrefix + "/clusters/default/namespaces/my-namespace/charts/categories")
			assert.NoError(t, err)
			defer res.Body.Close()

			assert.Equal(t, res.StatusCode, http.StatusOK, "http status code should match")

			var b bodyAPIListResponse
			json.NewDecoder(res.Body).Decode(&b)
			assert.Len(t, *b.Data, len(tt.expectedChartCategories))
		})
	}
}

// tests the GET /{apiVersion}/clusters/default/namespaces/{namespace}/charts/{repo}/categories endpoint
// particularly, it just tests that the endpoint is running the expected count query
func Test_GetChartCategoriesRepo(t *testing.T) {
	ts := httptest.NewServer(setupRoutes())
	defer ts.Close()

	tests := []struct {
		name                    string
		repo                    string
		expectedChartCategories []*models.ChartCategory
	}{
		{
			"no charts",
			"my-repo",
			[]*models.ChartCategory{},
		},
		{
			"two charts - same category",
			"my-repo",
			[]*models.ChartCategory{
				{Name: "cat1", Count: 1},
				{Name: "cat2", Count: 2},
				{Name: "cat3", Count: 3},
			},
		},
		{
			"two charts - different category",
			"my-repo",
			[]*models.ChartCategory{
				{Name: "cat1", Count: 1},
				{Name: "cat2", Count: 2},
				{Name: "cat3", Count: 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setMockManager(t)
			defer cleanup()

			rows := sqlmock.NewRows([]string{"name", "count"})
			for _, chartCategories := range tt.expectedChartCategories {
				rows.AddRow(chartCategories.Name, chartCategories.Count)
			}
			mock.ExpectQuery("SELECT (info ->> 'category')*").
				WithArgs("my-namespace", globalReposNamespace, tt.repo).
				WillReturnRows(rows)

			res, err := http.Get(ts.URL + pathPrefix + "/clusters/default/namespaces/my-namespace/charts/" + tt.repo + "/categories")
			assert.NoError(t, err)
			defer res.Body.Close()

			assert.Equal(t, res.StatusCode, http.StatusOK, "http status code should match")

			var b bodyAPIListResponse
			json.NewDecoder(res.Body).Decode(&b)
			assert.Len(t, *b.Data, len(tt.expectedChartCategories))
		})
	}
}

// tests the GET /{apiVersion}/clusters/default/namespaces/charts/{repo}/{chartName} endpoint
func Test_GetChartInRepo(t *testing.T) {
	ts := httptest.NewServer(setupRoutes())
	defer ts.Close()

	tests := []struct {
		name     string
		err      error
		chart    models.Chart
		wantCode int
	}{
		{
			"chart does not exist",
			errors.New("return an error when checking if chart exists"),
			models.Chart{Repo: testRepo, ID: "my-repo/my-chart"},
			http.StatusNotFound,
		},
		{
			"chart exists",
			nil,
			models.Chart{Repo: testRepo, ID: "my-repo/my-chart", ChartVersions: []models.ChartVersion{{Version: "0.1.0"}}},
			http.StatusOK,
		},
		{
			"chart has multiple versions",
			nil,
			models.Chart{Repo: testRepo, ID: "my-repo/my-chart", ChartVersions: []models.ChartVersion{{Version: "0.1.0"}, {Version: "0.0.1"}}},
			http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setMockManager(t)
			defer cleanup()

			mockQuery := mock.ExpectQuery("SELECT info FROM charts WHERE *").
				WithArgs("my-namespace", tt.chart.ID)

			if tt.err != nil {
				mockQuery.WillReturnError(tt.err)
			} else {
				chartJSON, err := json.Marshal(tt.chart)
				if err != nil {
					t.Fatalf("%+v", err)
				}
				mockQuery.WillReturnRows(sqlmock.NewRows([]string{"info"}).AddRow(chartJSON))
			}

			res, err := http.Get(ts.URL + pathPrefix + "/clusters/default/namespaces/my-namespace/charts/" + tt.chart.ID)
			assert.NoError(t, err)
			defer res.Body.Close()

			assert.Equal(t, res.StatusCode, tt.wantCode, "http status code should match")
		})
	}
}

// tests the GET /{apiVersion}/clusters/default/namespaces/charts/{repo}/{chartName}/versions endpoint
func Test_ListChartVersions(t *testing.T) {
	ts := httptest.NewServer(setupRoutes())
	defer ts.Close()

	tests := []struct {
		name     string
		err      error
		chart    models.Chart
		wantCode int
	}{
		{
			"chart does not exist",
			errors.New("return an error when checking if chart exists"),
			models.Chart{Repo: testRepo, ID: "my-repo/my-chart"},
			http.StatusNotFound,
		},
		{
			"chart exists",
			nil,
			models.Chart{Repo: testRepo, ID: "my-repo/my-chart", ChartVersions: []models.ChartVersion{{Version: "0.1.0"}}},
			http.StatusOK,
		},
		{
			"chart has multiple versions",
			nil,
			models.Chart{Repo: testRepo, ID: "my-repo/my-chart", ChartVersions: []models.ChartVersion{{Version: "0.1.0"}, {Version: "0.0.1"}}},
			http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setMockManager(t)
			defer cleanup()

			mockQuery := mock.ExpectQuery("SELECT info FROM charts WHERE *").
				WithArgs("my-namespace", tt.chart.ID)

			if tt.err != nil {
				mockQuery.WillReturnError(tt.err)
			} else {
				chartJSON, err := json.Marshal(tt.chart)
				if err != nil {
					t.Fatalf("%+v", err)
				}
				mockQuery.WillReturnRows(sqlmock.NewRows([]string{"info"}).AddRow(chartJSON))
			}

			res, err := http.Get(ts.URL + pathPrefix + "/clusters/default/namespaces/my-namespace/charts/" + tt.chart.ID + "/versions")
			assert.NoError(t, err)
			defer res.Body.Close()

			assert.Equal(t, res.StatusCode, tt.wantCode, "http status code should match")
		})
	}
}

// tests the GET /{apiVersion}/clusters/default/namespaces/charts/{repo}/{chartName}/versions/{:version} endpoint
func Test_GetChartVersion(t *testing.T) {
	ts := httptest.NewServer(setupRoutes())
	defer ts.Close()

	tests := []struct {
		name     string
		err      error
		chart    models.Chart
		wantCode int
	}{
		{
			"chart does not exist",
			errors.New("return an error when checking if chart exists"),
			models.Chart{Repo: testRepo, ID: "my-repo/my-chart", ChartVersions: []models.ChartVersion{{Version: "0.1.0"}}},
			http.StatusNotFound,
		},
		{
			"chart exists",
			nil,
			models.Chart{Repo: testRepo, ID: "my-repo/my-chart", ChartVersions: []models.ChartVersion{{Version: "0.1.0"}}},
			http.StatusOK,
		},
		{
			"chart has multiple versions",
			nil,
			models.Chart{Repo: testRepo, ID: "my-repo/my-chart", ChartVersions: []models.ChartVersion{{Version: "0.1.0"}, {Version: "0.0.1"}}},
			http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setMockManager(t)
			defer cleanup()

			mockQuery := mock.ExpectQuery("SELECT info FROM charts WHERE *").
				WithArgs("my-namespace", tt.chart.ID)

			if tt.err != nil {
				mockQuery.WillReturnError(tt.err)
			} else {
				chartJSON, err := json.Marshal(tt.chart)
				if err != nil {
					t.Fatalf("%+v", err)
				}
				mockQuery.WillReturnRows(sqlmock.NewRows([]string{"info"}).AddRow(chartJSON))
			}

			res, err := http.Get(ts.URL + pathPrefix + "/clusters/default/namespaces/my-namespace/charts/" + tt.chart.ID + "/versions/" + tt.chart.ChartVersions[0].Version)
			assert.NoError(t, err)
			defer res.Body.Close()

			assert.Equal(t, res.StatusCode, tt.wantCode, "http status code should match")
		})
	}
}

// tests both the GET /{apiVersion}/clusters/default/namespaces/{namespace}/assets/{repo}/{chartName}/logo-160x160-fit.png endpoint
// and the non-cluster /{apiVersion}/ns/{namespace}/assets/{repo}/{chartName}/logo-160x160-fit.png endpoint

func Test_GetChartIcon(t *testing.T) {
	ts := httptest.NewServer(setupRoutes())
	defer ts.Close()

	tests := []struct {
		name     string
		err      error
		chart    models.Chart
		wantCode int
	}{
		{
			"chart does not exist",
			errors.New("return an error when checking if chart exists"),
			models.Chart{ID: "my-repo/my-chart"},
			http.StatusNotFound,
		},
		{
			"chart has icon",
			nil,
			models.Chart{ID: "my-repo/my-chart", RawIcon: iconBytes()},
			http.StatusOK,
		},
		{
			"chart does not have a icon",
			nil,
			models.Chart{ID: "my-repo/my-chart"},
			http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setMockManager(t)
			defer cleanup()

			mockQuery := mock.ExpectQuery("SELECT info FROM charts WHERE *").
				WithArgs("my-namespace", tt.chart.ID)

			if tt.err != nil {
				mockQuery.WillReturnError(tt.err)
			} else {
				chartJSON, err := json.Marshal(tt.chart)
				if err != nil {
					t.Fatalf("%+v", err)
				}
				mockQuery.WillReturnRows(sqlmock.NewRows([]string{"info"}).AddRow(chartJSON))
			}

			path := "/clusters/default/namespaces/my-namespace/assets/"
			res, err := http.Get(ts.URL + pathPrefix + path + tt.chart.ID + "/logo")
			assert.NoError(t, err)
			defer res.Body.Close()

			assert.Equal(t, res.StatusCode, tt.wantCode, "http status code should match")
		})
	}
}

// tests the GET /{apiVersion}/clusters/default/namespaces/assets/{repo}/{chartName}/versions/{version}/README.md endpoint
func Test_GetChartReadme(t *testing.T) {
	ts := httptest.NewServer(setupRoutes())
	defer ts.Close()

	tests := []struct {
		name     string
		version  string
		err      error
		files    models.ChartFiles
		wantCode int
	}{
		{
			"chart does not exist",
			"0.1.0",
			errors.New("return an error when checking if chart exists"),
			models.ChartFiles{ID: "my-repo/my-chart"},
			http.StatusNotFound,
		},
		{
			"chart exists",
			"1.2.3",
			nil,
			models.ChartFiles{ID: "my-repo/my-chart", Readme: testChartReadme},
			http.StatusOK,
		},
		{
			"chart does not have a readme",
			"1.1.1",
			nil,
			models.ChartFiles{ID: "my-repo/my-chart"},
			http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setMockManager(t)
			defer cleanup()

			mockQuery := mock.ExpectQuery("SELECT info FROM files").
				WithArgs("my-namespace", tt.files.ID+"-"+tt.version)

			if tt.err != nil {
				mockQuery.WillReturnError(tt.err)
			} else {
				filesJSON, err := json.Marshal(tt.files)
				if err != nil {
					t.Fatalf("%+v", err)
				}
				mockQuery.WillReturnRows(sqlmock.NewRows([]string{"info"}).AddRow(filesJSON))
			}

			res, err := http.Get(ts.URL + pathPrefix + "/clusters/default/namespaces/my-namespace/assets/" + tt.files.ID + "/versions/" + tt.version + "/README.md")
			assert.NoError(t, err)
			defer res.Body.Close()

			assert.Equal(t, tt.wantCode, res.StatusCode, "http status code should match")
		})
	}
}

// tests the GET /{apiVersion}/clusters/default/namespaces/assets/{repo}/{chartName}/versions/{version}/values.yaml endpoint
func Test_GetChartValues(t *testing.T) {
	ts := httptest.NewServer(setupRoutes())
	defer ts.Close()

	tests := []struct {
		name     string
		version  string
		err      error
		files    models.ChartFiles
		wantCode int
	}{
		{
			"chart does not exist",
			"0.1.0",
			errors.New("return an error when checking if chart exists"),
			models.ChartFiles{ID: "my-repo/my-chart"},
			http.StatusNotFound,
		},
		{
			"chart exists",
			"3.2.1",
			nil,
			models.ChartFiles{ID: "my-repo/my-chart", Values: testChartValues},
			http.StatusOK,
		},
		{
			"chart does not have values.yaml",
			"2.2.2",
			nil,
			models.ChartFiles{ID: "my-repo/my-chart"},
			http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setMockManager(t)
			defer cleanup()

			mockQuery := mock.ExpectQuery("SELECT info FROM files").
				WithArgs("my-namespace", tt.files.ID+"-"+tt.version)

			if tt.err != nil {
				mockQuery.WillReturnError(tt.err)
			} else {
				filesJSON, err := json.Marshal(tt.files)
				if err != nil {
					t.Fatalf("%+v", err)
				}
				mockQuery.WillReturnRows(sqlmock.NewRows([]string{"info"}).AddRow(filesJSON))
			}

			res, err := http.Get(ts.URL + pathPrefix + "/clusters/default/namespaces/my-namespace/assets/" + tt.files.ID + "/versions/" + tt.version + "/values.yaml")
			assert.NoError(t, err)
			defer res.Body.Close()

			assert.Equal(t, res.StatusCode, tt.wantCode, "http status code should match")
		})
	}
}

// tests the GET /{apiVersion}/clusters/default/namespaces/assets/{repo}/{chartName}/versions/{version}/values/schema.json endpoint
func Test_GetChartSchema(t *testing.T) {
	ts := httptest.NewServer(setupRoutes())
	defer ts.Close()

	tests := []struct {
		name     string
		version  string
		err      error
		files    models.ChartFiles
		wantCode int
	}{
		{
			"chart does not exist",
			"0.1.0",
			errors.New("return an error when checking if chart exists"),
			models.ChartFiles{ID: "my-repo/my-chart"},
			http.StatusNotFound,
		},
		{
			"chart exists",
			"3.2.1",
			nil,
			models.ChartFiles{ID: "my-repo/my-chart", Schema: testChartSchema},
			http.StatusOK,
		},
		{
			"chart does not have values.schema.json",
			"2.2.2",
			nil,
			models.ChartFiles{ID: "my-repo/my-chart"},
			http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setMockManager(t)
			defer cleanup()

			mockQuery := mock.ExpectQuery("SELECT info FROM files").
				WithArgs("my-namespace", tt.files.ID+"-"+tt.version)

			if tt.err != nil {
				mockQuery.WillReturnError(tt.err)
			} else {
				filesJSON, err := json.Marshal(tt.files)
				if err != nil {
					t.Fatalf("%+v", err)
				}
				mockQuery.WillReturnRows(sqlmock.NewRows([]string{"info"}).AddRow(filesJSON))
			}

			res, err := http.Get(ts.URL + pathPrefix + "/clusters/default/namespaces/my-namespace/assets/" + tt.files.ID + "/versions/" + tt.version + "/values.schema.json")
			assert.NoError(t, err)
			defer res.Body.Close()

			assert.Equal(t, res.StatusCode, tt.wantCode, "http status code should match")
		})
	}
}
