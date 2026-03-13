import { useState, useEffect, useCallback } from "react";

const API_BASE = import.meta.env.VITE_API_BASE || "";

const fmt = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 0 });
const fmtShort = (n) => {
  if (n >= 1_000_000) return `$${(n / 1_000_000).toFixed(2)}M`;
  if (n >= 1_000) return `$${(n / 1_000).toFixed(0)}K`;
  return fmt.format(n);
};
const fmtDate = (s) => s ? new Date(s).toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric" }) : "—";

function StatCard({ label, value, sub, accent }) {
  return (
    <div style={{
      background: "white", borderRadius: 12, padding: "24px 28px",
      boxShadow: "0 1px 3px rgba(0,0,0,.08)", borderTop: `3px solid ${accent || "#1a56db"}`
    }}>
      <div style={{ fontSize: 13, color: "#6b7280", fontWeight: 500, letterSpacing: ".02em", textTransform: "uppercase" }}>{label}</div>
      <div style={{ fontSize: 32, fontWeight: 700, color: "#111827", margin: "8px 0 4px", letterSpacing: "-.02em" }}>{value}</div>
      {sub && <div style={{ fontSize: 13, color: "#9ca3af" }}>{sub}</div>}
    </div>
  );
}

function Badge({ status }) {
  const map = {
    ACTIVE:   { bg: "#d1fae5", color: "#065f46" },
    CHURNED:  { bg: "#fee2e2", color: "#991b1b" },
    EXPIRED:  { bg: "#fef3c7", color: "#92400e" },
  };
  const style = map[status] || { bg: "#f3f4f6", color: "#374151" };
  return (
    <span style={{
      display: "inline-block", padding: "2px 10px", borderRadius: 99,
      fontSize: 11, fontWeight: 600, letterSpacing: ".04em",
      background: style.bg, color: style.color
    }}>
      {status}
    </span>
  );
}

function buildGroups(contracts, asOf) {
  const map = {};
  for (const c of contracts) {
    const key = c.client_name || "—";
    if (!map[key]) map[key] = { name: key, contracts: [], totalARR: 0, activeCount: 0 };
    const isActive = c.is_evergreen || (c.contract_start_date <= asOf && c.contract_end_date >= asOf);
    map[key].contracts.push({ ...c, _isArrActive: isActive });
    if (isActive) { map[key].totalARR += c.arr_usd || 0; map[key].activeCount++; }
  }
  return Object.values(map).sort((a, b) => b.totalARR - a.totalARR);
}

export default function App() {
  const [summary, setSummary]     = useState(null);
  const [contracts, setContracts] = useState([]);
  const [status, setStatus]       = useState("ACTIVE");
  const [loading, setLoading]     = useState(true);
  const [syncing, setSyncing]     = useState(false);
  const [syncMsg, setSyncMsg]     = useState(null);
  const [search, setSearch]       = useState("");
  const [asOf, setAsOf]           = useState(new Date().toISOString().split("T")[0]);
  const [expanded, setExpanded]   = useState({});
  const [expandAll, setExpandAll] = useState(false);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [sumRes, conRes] = await Promise.all([
        fetch(`${API_BASE}/api/summary?as_of=${asOf}`),
        fetch(`${API_BASE}/api/contracts?status=${status}&as_of=${asOf}`),
      ]);
      setSummary(await sumRes.json());
      const conData = await conRes.json();
      setContracts(conData.contracts || []);
    } catch (e) {
      console.error("Fetch error", e);
    } finally {
      setLoading(false);
    }
  }, [status, asOf]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const handleSync = async (full = false) => {
    setSyncing(true);
    setSyncMsg(null);
    try {
      const res = await fetch(`${API_BASE}/api/sync?full=${full}`, { method: "POST" });
      const data = await res.json();
      if (res.ok) {
        setSyncMsg(`✓ Synced ${data.upserted} contracts (${data.incremental ? "incremental" : "full"})`);
        fetchData();
      } else {
        setSyncMsg(`✗ ${data.error}`);
      }
    } catch (e) {
      setSyncMsg("✗ Network error");
    } finally {
      setSyncing(false);
      setTimeout(() => setSyncMsg(null), 6000);
    }
  };

  const filtered = (contracts || []).filter(c => {
    if (!search) return true;
    const q = search.toLowerCase();
    return (
      c.client_name?.toLowerCase().includes(q) ||
      c.deal_name?.toLowerCase().includes(q) ||
      c.deal_id?.toLowerCase().includes(q) ||
      c.opportunity_id?.toLowerCase().includes(q)
    );
  });

  const groups = buildGroups(filtered, asOf);
  const totalARR = groups.reduce((s, g) => s + g.totalARR, 0);
  const totalContracts = filtered.filter(c =>
    c.is_evergreen || (c.contract_start_date <= asOf && c.contract_end_date >= asOf)
  ).length;

  const isOpen = (name) => expanded[name] ?? false;

  const toggleExpand = (name) => setExpanded(prev => ({ ...prev, [name]: !prev[name] }));

  const handleExpandAll = () => {
    const next = !expandAll;
    setExpandAll(next);
    const state = {};
    groups.forEach(g => { state[g.name] = next; });
    setExpanded(state);
  };

  const thStyle = {
    padding: "11px 16px", fontWeight: 600, color: "#374151",
    fontSize: 12, letterSpacing: ".04em", textTransform: "uppercase",
    whiteSpace: "nowrap", userSelect: "none", background: "#f9fafb",
  };

  return (
    <div style={{ minHeight: "100vh", background: "#f9fafb", fontFamily: "Inter, system-ui, sans-serif" }}>
      {/* Header */}
      <div style={{ background: "white", borderBottom: "1px solid #e5e7eb", padding: "0 32px" }}>
        <div style={{ maxWidth: 1400, margin: "0 auto", display: "flex", alignItems: "center", justifyContent: "space-between", height: 64 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <div style={{ width: 8, height: 8, borderRadius: "50%", background: "#1a56db" }} />
            <span style={{ fontWeight: 700, fontSize: 16, color: "#111827" }}>Coder ARR Tracker</span>
            <span style={{ fontSize: 12, color: "#9ca3af", marginLeft: 4 }}>via Campfire</span>
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
              <span style={{ fontSize: 13, color: "#6b7280", fontWeight: 500 }}>ARR as of</span>
              <input
                type="date" value={asOf}
                onChange={e => setAsOf(e.target.value)}
                style={{ padding: "7px 12px", borderRadius: 8, border: "1px solid #d1d5db", fontSize: 13, color: "#111827", background: "white", cursor: "pointer", outline: "none" }}
              />
              <button
                onClick={() => setAsOf(new Date().toISOString().split("T")[0])}
                style={{ padding: "7px 12px", borderRadius: 8, border: "1px solid #d1d5db", fontSize: 12, color: "#6b7280", background: "white", cursor: "pointer" }}
              >
                Today
              </button>
            </div>
            <div style={{ width: 1, height: 24, background: "#e5e7eb", margin: "0 4px" }} />
            {syncMsg && <span style={{ fontSize: 13, color: syncMsg.startsWith("✓") ? "#065f46" : "#991b1b", fontWeight: 500 }}>{syncMsg}</span>}
            <button onClick={() => handleSync(false)} disabled={syncing}
              style={{ background: syncing ? "#e5e7eb" : "#1a56db", color: syncing ? "#6b7280" : "white", border: "none", borderRadius: 8, padding: "8px 16px", fontSize: 13, fontWeight: 600, cursor: syncing ? "not-allowed" : "pointer" }}>
              {syncing ? "Syncing…" : "↻ Sync Now"}
            </button>
            <button onClick={() => handleSync(true)} disabled={syncing}
              style={{ background: "white", color: "#374151", border: "1px solid #d1d5db", borderRadius: 8, padding: "8px 16px", fontSize: 13, fontWeight: 500, cursor: syncing ? "not-allowed" : "pointer" }}>
              Full Sync
            </button>
          </div>
        </div>
      </div>

      <div style={{ maxWidth: 1400, margin: "0 auto", padding: "32px 32px" }}>

        {/* Summary cards */}
        {summary && (
          <div style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: 20, marginBottom: 32 }}>
            <StatCard label="Total ARR (USD)" value={fmtShort(summary.total_arr_usd)} sub={`${fmt.format(summary.total_arr_usd)} · as of ${asOf}`} accent="#1a56db" />
            <StatCard label="Implied MRR (USD)" value={fmtShort(summary.total_mrr_usd)} sub="ARR ÷ 12" accent="#7c3aed" />
            <StatCard label="Active Contracts" value={summary.active_contracts} sub={`${summary.evergreen_contracts} evergreen`} accent="#059669" />
            <StatCard label="Last Synced" value={summary.last_synced_at ? fmtDate(summary.last_synced_at) : "Never"} sub="Auto-refreshes every 24h" accent="#f59e0b" />
          </div>
        )}

        {/* Currency breakdown */}
        {summary?.by_currency?.length > 1 && (
          <div style={{ background: "white", borderRadius: 12, padding: "20px 28px", boxShadow: "0 1px 3px rgba(0,0,0,.08)", marginBottom: 28, display: "flex", gap: 32, flexWrap: "wrap", alignItems: "center" }}>
            <span style={{ fontSize: 13, fontWeight: 600, color: "#374151" }}>ARR by Currency</span>
            {summary.by_currency.map(c => (
              <div key={c.currency} style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <span style={{ background: "#eff6ff", color: "#1d4ed8", borderRadius: 6, padding: "2px 8px", fontSize: 12, fontWeight: 700 }}>{c.currency}</span>
                <span style={{ fontSize: 14, fontWeight: 600, color: "#111827" }}>{fmtShort(c.arr_usd)}</span>
                <span style={{ fontSize: 12, color: "#9ca3af" }}>({c.count})</span>
              </div>
            ))}
          </div>
        )}

        {/* Filters */}
        <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 16, flexWrap: "wrap" }}>
          <input
            value={search} onChange={e => setSearch(e.target.value)}
            placeholder="Search customer, deal, opportunity ID…"
            style={{ flex: "1 1 280px", padding: "9px 14px", borderRadius: 8, border: "1px solid #d1d5db", fontSize: 14, color: "#111827", outline: "none", background: "white" }}
          />
          {["ACTIVE", "CHURNED", "ALL"].map(s => (
            <button key={s} onClick={() => setStatus(s)}
              style={{ padding: "8px 16px", borderRadius: 8, fontSize: 13, fontWeight: 500, cursor: "pointer", border: "1px solid", background: status === s ? "#1a56db" : "white", color: status === s ? "white" : "#374151", borderColor: status === s ? "#1a56db" : "#d1d5db" }}>
              {s}
            </button>
          ))}
          <button onClick={handleExpandAll}
            style={{ padding: "8px 14px", borderRadius: 8, fontSize: 13, fontWeight: 500, cursor: "pointer", border: "1px solid #d1d5db", background: "white", color: "#374151" }}>
            {expandAll ? "Collapse All" : "Expand All"}
          </button>
          <span style={{ fontSize: 13, color: "#9ca3af", marginLeft: "auto" }}>
            {groups.length} customer{groups.length !== 1 ? "s" : ""} · {totalContracts} active contract{totalContracts !== 1 ? "s" : ""}
          </span>
        </div>

        {/* Table */}
        <div style={{ background: "white", borderRadius: 12, boxShadow: "0 1px 3px rgba(0,0,0,.08)", overflow: "hidden" }}>
          {loading ? (
            <div style={{ padding: 48, textAlign: "center", color: "#9ca3af" }}>Loading…</div>
          ) : groups.length === 0 ? (
            <div style={{ padding: 48, textAlign: "center", color: "#9ca3af" }}>No contracts found</div>
          ) : (
            <div style={{ overflowX: "auto" }}>
              <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
                <thead>
                  <tr style={{ borderBottom: "1px solid #e5e7eb" }}>
                    <th style={{ ...thStyle, width: 40 }}></th>
                    <th style={{ ...thStyle, textAlign: "left" }}>Customer</th>
                    <th style={{ ...thStyle, textAlign: "left" }}>Deal</th>
                    <th style={{ ...thStyle, textAlign: "center" }}>CCY</th>
                    <th style={{ ...thStyle, textAlign: "left" }}>Start</th>
                    <th style={{ ...thStyle, textAlign: "left" }}>End</th>
                    <th style={{ ...thStyle, textAlign: "right" }}>Days</th>
                    <th style={{ ...thStyle, textAlign: "right" }}>TCV</th>
                    <th style={{ ...thStyle, textAlign: "right" }}>ARR (native)</th>
                    <th style={{ ...thStyle, textAlign: "right" }}>ARR (USD)</th>
                    <th style={{ ...thStyle, textAlign: "center" }}>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {groups.map((group) => {
                    const open = isOpen(group.name);
                    const multi = group.contracts.length > 1;
                    const solo = group.contracts[0];
                    return (
                      <>
                        {/* Customer group row */}
                        <tr
                          key={`grp-${group.name}`}
                          onClick={() => multi && toggleExpand(group.name)}
                          style={{ borderBottom: "1px solid #e5e7eb", background: "#eff6ff", cursor: multi ? "pointer" : "default" }}
                        >
                          <td style={{ padding: "13px 8px 13px 16px", color: "#6b7280", fontSize: 14, textAlign: "center" }}>
                            {multi ? (open ? "▾" : "▸") : ""}
                          </td>
                          <td style={{ padding: "13px 4px 13px 0", fontWeight: 700, color: "#1e3a8a" }}>
                            {group.name}
                            {multi && (
                              <span style={{ marginLeft: 8, fontSize: 11, fontWeight: 500, background: "#dbeafe", color: "#1d4ed8", borderRadius: 99, padding: "2px 8px" }}>
                                {group.contracts.length} deals
                              </span>
                            )}
                          </td>
                          <td style={{ padding: "13px 16px", color: "#374151", maxWidth: 200, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                            {!multi ? (solo?.deal_name || "—") : ""}
                          </td>
                          <td style={{ padding: "13px 16px", textAlign: "center" }}>
                            {!multi && <span style={{ background: solo?.currency !== "USD" ? "#eff6ff" : "#f3f4f6", color: solo?.currency !== "USD" ? "#1d4ed8" : "#374151", borderRadius: 4, padding: "2px 6px", fontSize: 11, fontWeight: 700 }}>{solo?.currency}</span>}
                          </td>
                          <td style={{ padding: "13px 16px", color: "#6b7280", whiteSpace: "nowrap" }}>{!multi ? fmtDate(solo?.contract_start_date) : ""}</td>
                          <td style={{ padding: "13px 16px", color: "#6b7280", whiteSpace: "nowrap" }}>{!multi ? (solo?.is_evergreen ? "Evergreen" : fmtDate(solo?.contract_end_date)) : ""}</td>
                          <td style={{ padding: "13px 16px", textAlign: "right", color: "#6b7280" }}>{!multi && solo?.contract_days > 0 ? Math.round(solo.contract_days) : ""}</td>
                          <td style={{ padding: "13px 16px", textAlign: "right", color: "#374151" }}>{!multi && solo?.total_contract_value > 0 ? fmt.format(solo.total_contract_value) : ""}</td>
                          <td style={{ padding: "13px 16px", textAlign: "right", color: "#374151" }}>{!multi && solo?.arr > 0 ? fmt.format(solo.arr) : ""}</td>
                          <td style={{ padding: "13px 16px", textAlign: "right", fontWeight: 700, color: "#1e3a8a" }}>
                            {group.totalARR > 0 ? fmt.format(group.totalARR) : "—"}
                          </td>
                          <td style={{ padding: "13px 16px", textAlign: "center" }}>
                            {!multi && <Badge status={solo?.status} />}
                          </td>
                        </tr>

                        {/* Expanded deal rows */}
                        {multi && open && group.contracts.map((c, i) => (
                          <tr
                            key={`deal-${c.campfire_id}`}
                            style={{
                              borderBottom: i === group.contracts.length - 1 ? "1px solid #e5e7eb" : "1px solid #f3f4f6",
                              background: c._isArrActive ? "white" : "#fafafa",
                              opacity: c._isArrActive ? 1 : 0.5,
                            }}
                          >
                            <td style={{ padding: "10px 16px", textAlign: "center", color: "#d1d5db", fontSize: 12 }}>↳</td>
                            <td style={{ padding: "10px 4px" }} />
                            <td style={{ padding: "10px 16px", color: "#374151", maxWidth: 200, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{c.deal_name || "—"}</td>
                            <td style={{ padding: "10px 16px", textAlign: "center" }}>
                              <span style={{ background: c.currency !== "USD" ? "#eff6ff" : "#f3f4f6", color: c.currency !== "USD" ? "#1d4ed8" : "#374151", borderRadius: 4, padding: "2px 6px", fontSize: 11, fontWeight: 700 }}>{c.currency}</span>
                            </td>
                            <td style={{ padding: "10px 16px", color: "#6b7280", whiteSpace: "nowrap" }}>{fmtDate(c.contract_start_date)}</td>
                            <td style={{ padding: "10px 16px", color: "#6b7280", whiteSpace: "nowrap" }}>{c.is_evergreen ? "Evergreen" : fmtDate(c.contract_end_date)}</td>
                            <td style={{ padding: "10px 16px", textAlign: "right", color: "#6b7280" }}>{c.contract_days > 0 ? Math.round(c.contract_days) : "—"}</td>
                            <td style={{ padding: "10px 16px", textAlign: "right", color: "#374151" }}>{c.total_contract_value > 0 ? fmt.format(c.total_contract_value) : "—"}</td>
                            <td style={{ padding: "10px 16px", textAlign: "right", color: "#374151" }}>{c.arr > 0 ? fmt.format(c.arr) : "—"}</td>
                            <td style={{ padding: "10px 16px", textAlign: "right", fontWeight: 600, color: "#111827" }}>
                              {c._isArrActive && c.arr_usd > 0 ? fmt.format(c.arr_usd) : "—"}
                            </td>
                            <td style={{ padding: "10px 16px", textAlign: "center" }}><Badge status={c.status} /></td>
                          </tr>
                        ))}
                      </>
                    );
                  })}
                </tbody>
                <tfoot>
                  <tr style={{ background: "#f9fafb", borderTop: "2px solid #e5e7eb" }}>
                    <td colSpan={9} style={{ padding: "12px 16px", fontWeight: 700, color: "#374151", fontSize: 13 }}>
                      TOTAL — ARR as of {asOf} · {groups.length} customers · {totalContracts} active contracts
                    </td>
                    <td style={{ padding: "12px 16px", textAlign: "right", fontWeight: 700, color: "#111827", fontSize: 14 }}>
                      {fmt.format(totalARR)}
                    </td>
                    <td />
                  </tr>
                </tfoot>
              </table>
            </div>
          )}
        </div>

        <div style={{ marginTop: 16, fontSize: 12, color: "#9ca3af", textAlign: "right" }}>
          ARR = Total Contract Value ÷ Contract Days × 365 &nbsp;·&nbsp;
          Non-USD converted at signing-date spot rate &nbsp;·&nbsp;
          Auto-refreshes every 24h
        </div>
      </div>
    </div>
  );
}
