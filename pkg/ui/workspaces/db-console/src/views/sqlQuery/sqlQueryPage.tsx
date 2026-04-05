// Copyright 2024 Oxide Computer Company
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

import React, { useState, useCallback } from "react";
import { Helmet } from "react-helmet";

interface SqlColumn {
  name: string;
  type: string;
  oid: number;
}

interface SqlTxnResult {
  tag: string;
  columns?: SqlColumn[];
  rows?: Record<string, unknown>[];
  error?: { message: string; code: string; severity: string };
  statement: number;
}

interface SqlExecution {
  txn_results: SqlTxnResult[];
}

interface SqlResponse {
  num_statements?: number;
  execution?: SqlExecution;
  error?: { message: string; code: string; severity: string };
  request_time?: string;
}

const SQLQueryPage: React.FC = () => {
  const [query, setQuery] = useState("SELECT 1 AS result;");
  const [database, setDatabase] = useState("defaultdb");
  const [results, setResults] = useState<SqlResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const executeQuery = useCallback(async () => {
    setLoading(true);
    setError(null);
    setResults(null);

    try {
      const resp = await fetch("/api/v2/sql/", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          database: database,
          execute: true,
          statements: [{ sql: query }],
        }),
      });

      const data: SqlResponse = await resp.json();
      if (!resp.ok) {
        setError(data.error?.message || `HTTP ${resp.status}`);
      } else if (data.error) {
        setError(data.error.message);
      } else {
        setResults(data);
      }
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, [query, database]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === "Enter") {
        executeQuery();
      }
    },
    [executeQuery],
  );

  return (
    <div>
      <Helmet title="SQL Query" />
      <h3 style={{ fontSize: 20, fontWeight: 500, marginBottom: 16 }}>SQL Query</h3>
      <div style={{ marginBottom: 12 }}>
        <label style={{ marginRight: 8, fontWeight: 600 }}>Database:</label>
        <input
          type="text"
          value={database}
          onChange={e => setDatabase(e.target.value)}
          style={{
            padding: "4px 8px",
            border: "1px solid #c0c6d9",
            borderRadius: 3,
            width: 200,
          }}
        />
      </div>
      <textarea
        value={query}
        onChange={e => setQuery(e.target.value)}
        onKeyDown={handleKeyDown}
        rows={8}
        style={{
          width: "100%",
          fontFamily: "SFMono-Regular, Menlo, Monaco, Consolas, monospace",
          fontSize: 13,
          padding: 12,
          border: "1px solid #c0c6d9",
          borderRadius: 3,
          resize: "vertical",
          boxSizing: "border-box",
        }}
        placeholder="Enter SQL query..."
      />
      <div style={{ marginTop: 8, marginBottom: 16 }}>
        <button
          onClick={executeQuery}
          disabled={loading || !query.trim()}
          style={{
            padding: "8px 24px",
            backgroundColor: "#394455",
            color: "#fff",
            border: "none",
            borderRadius: 3,
            cursor: loading ? "wait" : "pointer",
            fontSize: 14,
            fontWeight: 600,
          }}
        >
          {loading ? "Running..." : "Run Query"}
        </button>
        <span
          style={{ marginLeft: 12, color: "#8e96a0", fontSize: 12 }}
        >
          Ctrl+Enter / Cmd+Enter to execute
        </span>
      </div>

      {error && (
        <div
          style={{
            padding: 12,
            backgroundColor: "#fde8e8",
            border: "1px solid #f5a6a6",
            borderRadius: 3,
            color: "#7a1f1f",
            marginBottom: 16,
            fontFamily: "SFMono-Regular, Menlo, Monaco, Consolas, monospace",
            fontSize: 13,
            whiteSpace: "pre-wrap",
          }}
        >
          {error}
        </div>
      )}

      {results?.execution?.txn_results?.map((txnResult, i) => (
        <div key={i} style={{ marginBottom: 16 }}>
          {txnResult.error ? (
            <div
              style={{
                padding: 12,
                backgroundColor: "#fde8e8",
                border: "1px solid #f5a6a6",
                borderRadius: 3,
                color: "#7a1f1f",
                fontFamily:
                  "SFMono-Regular, Menlo, Monaco, Consolas, monospace",
                fontSize: 13,
              }}
            >
              {txnResult.error.message}
            </div>
          ) : txnResult.columns && txnResult.rows ? (
            <div>
              <div
                style={{
                  marginBottom: 4,
                  color: "#8e96a0",
                  fontSize: 12,
                }}
              >
                {txnResult.tag} &mdash; {txnResult.rows.length} row
                {txnResult.rows.length !== 1 ? "s" : ""}
              </div>
              <div style={{ overflow: "auto" }}>
                <table
                  style={{
                    borderCollapse: "collapse",
                    width: "100%",
                    fontSize: 13,
                    fontFamily:
                      "SFMono-Regular, Menlo, Monaco, Consolas, monospace",
                  }}
                >
                  <thead>
                    <tr>
                      {txnResult.columns.map((col, j) => (
                        <th
                          key={j}
                          style={{
                            textAlign: "left",
                            padding: "6px 12px",
                            borderBottom: "2px solid #c0c6d9",
                            backgroundColor: "#f5f7fa",
                            whiteSpace: "nowrap",
                          }}
                        >
                          {col.name}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {txnResult.rows.map((row, ri) => (
                      <tr key={ri}>
                        {txnResult.columns.map((col, ci) => {
                          const cell = row[col.name];
                          return (
                            <td
                              key={ci}
                              style={{
                                padding: "4px 12px",
                                borderBottom: "1px solid #e8ecf2",
                                whiteSpace: "pre-wrap",
                              }}
                            >
                              {cell === null || cell === undefined ? (
                                <span style={{ color: "#8e96a0" }}>NULL</span>
                              ) : (
                                String(cell)
                              )}
                            </td>
                          );
                        })}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          ) : (
            <div style={{ color: "#8e96a0", fontSize: 13 }}>
              {txnResult.tag}
            </div>
          )}
        </div>
      ))}
    </div>
  );
};

export default SQLQueryPage;
