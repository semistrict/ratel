// Copyright 2026 The Ratel Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunRatelDeployDirectHTTP(t *testing.T) {
	oldCompatDate := ratelDeployCompatDate
	oldConfig := ratelDeployConfig
	oldDOClasses := ratelDeployDOClasses
	oldName := ratelDeployName
	defer func() {
		ratelDeployCompatDate = oldCompatDate
		ratelDeployConfig = oldConfig
		ratelDeployDOClasses = oldDOClasses
		ratelDeployName = oldName
	}()
	ratelDeployCompatDate = "2024-04-01"
	ratelDeployConfig = ""
	ratelDeployDOClasses = []string{"ChatRoom"}
	ratelDeployName = "chat"

	var gotPath string
	var gotContentType string
	var gotCompatDate string
	var gotBindings string
	var gotBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		gotCompatDate = r.Header.Get("X-Compat-Date")
		gotBindings = r.Header.Get("X-Bindings")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, err = io.WriteString(w, `{"name":"counter","version":7}`)
		require.NoError(t, err)
	}))
	defer srv.Close()

	jsPath := filepath.Join(t.TempDir(), "counter.js")
	require.NoError(t, os.WriteFile(jsPath, []byte("export default {};"), 0644))

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	require.NoError(t, runRatelDeploy(nil, []string{u.Host, jsPath}))
	require.Equal(t, "/api/v2/workers/chat/", gotPath)
	require.Equal(t, "application/javascript", gotContentType)
	require.Equal(t, "2024-04-01", gotCompatDate)
	require.JSONEq(t, `{"durable_objects":[{"class_name":"ChatRoom"}]}`, gotBindings)
	require.Equal(t, "export default {};", gotBody)
}

func TestRunRatelDeployWorkerJSONC(t *testing.T) {
	oldCompatDate := ratelDeployCompatDate
	oldConfig := ratelDeployConfig
	oldDOClasses := ratelDeployDOClasses
	oldName := ratelDeployName
	defer func() {
		ratelDeployCompatDate = oldCompatDate
		ratelDeployConfig = oldConfig
		ratelDeployDOClasses = oldDOClasses
		ratelDeployName = oldName
	}()
	ratelDeployCompatDate = ""
	ratelDeployConfig = ""
	ratelDeployDOClasses = nil
	ratelDeployName = ""

	var gotPath string
	var gotContentType string
	var gotMetadata map[string]interface{}
	var gotAssetBody string
	var gotScriptBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		require.Empty(t, r.Header.Get("X-Compat-Date"))
		require.Empty(t, r.Header.Get("X-Bindings"))
		mr, err := r.MultipartReader()
		require.NoError(t, err)
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			body, err := io.ReadAll(part)
			require.NoError(t, err)
			switch part.FormName() {
			case "script":
				gotScriptBody = string(body)
			case "metadata":
				require.NoError(t, json.Unmarshal(body, &gotMetadata))
			case "asset":
				gotAssetBody = string(body)
			default:
				t.Fatalf("unexpected form field %q", part.FormName())
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, err = io.WriteString(w, `{"name":"chat","version":8}`)
		require.NoError(t, err)
	}))
	defer srv.Close()

	dir := t.TempDir()
	assetsDir := filepath.Join(dir, "assets")
	require.NoError(t, os.Mkdir(assetsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, "index.html"), []byte("<h1>chat</h1>"), 0644))
	jsPath := filepath.Join(dir, "chat_worker.js")
	require.NoError(t, os.WriteFile(jsPath, []byte("export default {};"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "worker.jsonc"), []byte(`{
		// Ratel worker metadata.
		"name": "chat",
		"compatibility_date": "2024-04-01",
		"assets": { "directory": "assets" },
		"durable_objects": {
			"bindings": [
				{ "name": "CHAT_ROOM", "class_name": "ChatRoom" },
			],
		},
	}`), 0644))

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	require.NoError(t, runRatelDeploy(nil, []string{u.Host, jsPath}))
	require.Equal(t, "/api/v2/workers/chat/", gotPath)
	require.True(t, strings.HasPrefix(gotContentType, "multipart/form-data; boundary="))
	require.Equal(t, "export default {};", gotScriptBody)
	require.Equal(t, "<h1>chat</h1>", gotAssetBody)
	require.Equal(t, "2024-04-01", gotMetadata["compat_date"])
	require.NotContains(t, gotMetadata, "data_base64")
	bindings, ok := gotMetadata["bindings"].(map[string]interface{})
	require.True(t, ok)
	dos := bindings["durable_objects"].([]interface{})
	require.Equal(t, "ChatRoom", dos[0].(map[string]interface{})["class_name"])
	assets := gotMetadata["assets"].([]interface{})
	asset := assets[0].(map[string]interface{})
	require.Equal(t, "/index.html", asset["path"])
	require.Equal(t, "text/html; charset=utf-8", asset["content_type"])
	require.Equal(t, float64(13), asset["size"])
	require.NotEmpty(t, asset["etag"])
}

func TestParseRatelWorkerJSONCKeepsCommentLikeStringContent(t *testing.T) {
	cfg, err := parseRatelWorkerJSONC([]byte(`{
		"name": "chat//room",
		"compat_date": "2024-04-01",
		"assets": "./assets",
		"do_classes": ["ChatRoom"],
		"durable_objects": [
			{ "class_name": "OtherRoom" },
		],
	}`))
	require.NoError(t, err)
	require.Equal(t, "chat//room", cfg.Name)
	require.Equal(t, "2024-04-01", cfg.CompatibilityDate)
	require.Equal(t, "./assets", cfg.AssetsDir)
	require.Equal(t, []string{"ChatRoom", "OtherRoom"}, cfg.DOClasses)
}

func TestNormalizeRatelWorkerAssetPathRejectsInvalidPaths(t *testing.T) {
	for _, path := range []string{"", ".", "..", "dir/../secret.txt", "dir/./file.txt"} {
		_, err := normalizeRatelWorkerAssetPath(path)
		require.Error(t, err, path)
	}

	path, err := normalizeRatelWorkerAssetPath("dir/file.txt")
	require.NoError(t, err)
	require.Equal(t, "/dir/file.txt", path)
}
