import { useState, useEffect } from 'react';
import { 
  Activity, Shield, Server, Network, Key, Zap, Lock, 
  Database, RefreshCw, Plus, Trash2, CheckCircle2
} from 'lucide-react';
import type { 
  Upstream, Route, APIKey, RateLimitPolicy, 
  AnalyticsOverview, RequestLog, AuditLog 
} from './types';

const ADMIN_URL = (import.meta as any).env?.VITE_ADMIN_URL || 'http://localhost:8081';
const ADMIN_KEY = (import.meta as any).env?.VITE_ADMIN_KEY || 'admin-secret-gateflow';

export default function App() {
  const [tab, setTab] = useState<'dashboard' | 'routes' | 'upstreams' | 'keys' | 'ratelimits' | 'logs' | 'audit'>('dashboard');
  const [overview, setOverview] = useState<AnalyticsOverview | null>(null);
  const [routes, setRoutes] = useState<Route[]>([]);
  const [upstreams, setUpstreams] = useState<Upstream[]>([]);
  const [apiKeys, setApiKeys] = useState<APIKey[]>([]);
  const [rateLimits, setRateLimits] = useState<RateLimitPolicy[]>([]);
  const [requestLogs, setRequestLogs] = useState<RequestLog[]>([]);
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(false);

  // Modal forms
  const [newKeyModal, setNewKeyModal] = useState(false);
  const [keyOwner, setKeyOwner] = useState('');
  const [keyDesc, setKeyDesc] = useState('');
  const [generatedKeyResult, setGeneratedKeyResult] = useState<string | null>(null);

  const [newRouteModal, setNewRouteModal] = useState(false);
  const [routeName, setRouteName] = useState('');
  const [routePath, setRoutePath] = useState('');
  const [routeUpstream, setRouteUpstream] = useState('');
  const [routeAuth, setRouteAuth] = useState(false);

  const headers = {
    'Content-Type': 'application/json',
    'X-Admin-Key': ADMIN_KEY,
  };

  const fetchData = async () => {
    setLoading(true);
    try {
      const [ovRes, rtRes, upRes, keyRes, rlRes, logRes, audRes] = await Promise.all([
        fetch(`${ADMIN_URL}/api/v1/analytics/overview`, { headers }),
        fetch(`${ADMIN_URL}/api/v1/routes`, { headers }),
        fetch(`${ADMIN_URL}/api/v1/upstreams`, { headers }),
        fetch(`${ADMIN_URL}/api/v1/keys`, { headers }),
        fetch(`${ADMIN_URL}/api/v1/rate-limits`, { headers }),
        fetch(`${ADMIN_URL}/api/v1/analytics/logs`, { headers }),
        fetch(`${ADMIN_URL}/api/v1/audit-logs`, { headers }),
      ]);

      if (ovRes.ok) setOverview(await ovRes.json());
      if (rtRes.ok) setRoutes(await rtRes.json());
      if (upRes.ok) setUpstreams(await upRes.json());
      if (keyRes.ok) setApiKeys(await keyRes.json());
      if (rlRes.ok) setRateLimits(await rlRes.json());
      if (logRes.ok) setRequestLogs(await logRes.json());
      if (audRes.ok) setAuditLogs(await audRes.json());
    } catch (e) {
      console.error('Failed to fetch from backend', e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 4000);
    return () => clearInterval(interval);
  }, []);

  const handleCreateKey = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const res = await fetch(`${ADMIN_URL}/api/v1/keys`, {
        method: 'POST',
        headers,
        body: JSON.stringify({
          owner: keyOwner,
          description: keyDesc,
          scopes: ['read', 'write'],
        }),
      });
      if (res.ok) {
        const data = await res.json();
        setGeneratedKeyResult(data.raw_key);
        fetchData();
      }
    } catch (err) {
      console.error(err);
    }
  };

  const handleRevokeKey = async (id: string) => {
    if (!confirm('Revoke this API Key immediately?')) return;
    await fetch(`${ADMIN_URL}/api/v1/keys/${id}`, { method: 'DELETE', headers });
    fetchData();
  };

  const handleCreateRoute = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const res = await fetch(`${ADMIN_URL}/api/v1/routes`, {
        method: 'POST',
        headers,
        body: JSON.stringify({
          name: routeName,
          path_pattern: routePath,
          methods: ['GET', 'POST'],
          upstream_id: routeUpstream || (upstreams[0]?.id || ''),
          auth_required: routeAuth,
          active: true,
          priority: 10,
        }),
      });
      if (res.ok) {
        setNewRouteModal(false);
        setRouteName('');
        setRoutePath('');
        fetchData();
      }
    } catch (err) {
      console.error(err);
    }
  };

  const handleDeleteRoute = async (id: string) => {
    if (!confirm('Delete route?')) return;
    await fetch(`${ADMIN_URL}/api/v1/routes/${id}`, { method: 'DELETE', headers });
    fetchData();
  };

  return (
    <div style={{ display: 'flex', minHeight: '100vh', backgroundColor: '#0a0d14', color: '#e2e8f0', fontFamily: 'system-ui, -apple-system, sans-serif' }}>
      {/* Sidebar */}
      <aside style={{ width: 260, backgroundColor: '#0f1422', borderRight: '1px solid #1e293b', padding: 24, display: 'flex', flexDirection: 'column', gap: 24 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <div style={{ width: 36, height: 36, borderRadius: 8, backgroundColor: '#0ea5e9', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Network size={22} color="#ffffff" />
          </div>
          <div>
            <div style={{ fontWeight: 700, fontSize: 18, color: '#f8fafc', letterSpacing: -0.5 }}>GateFlow</div>
            <div style={{ fontSize: 11, color: '#38bdf8', fontWeight: 600 }}>API Gateway Engine</div>
          </div>
        </div>

        <nav style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          {[
            { id: 'dashboard', label: 'Dashboard', icon: Activity },
            { id: 'routes', label: 'Routes & Policies', icon: Network },
            { id: 'upstreams', label: 'Upstream Clusters', icon: Server },
            { id: 'keys', label: 'API Keys', icon: Key },
            { id: 'ratelimits', label: 'Rate Limits', icon: Zap },
            { id: 'logs', label: 'Traffic Logs', icon: Database },
            { id: 'audit', label: 'Audit Trail', icon: Shield },
          ].map(item => {
            const Icon = item.icon;
            const active = tab === item.id;
            return (
              <button
                key={item.id}
                onClick={() => setTab(item.id as any)}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 12,
                  padding: '10px 14px',
                  borderRadius: 8,
                  border: 'none',
                  backgroundColor: active ? '#1e293b' : 'transparent',
                  color: active ? '#38bdf8' : '#94a3b8',
                  fontSize: 14,
                  fontWeight: active ? 600 : 500,
                  cursor: 'pointer',
                  textAlign: 'left',
                  transition: 'all 0.15s'
                }}
              >
                <Icon size={18} />
                {item.label}
              </button>
            );
          })}
        </nav>

        <div style={{ marginTop: 'auto', borderTop: '1px solid #1e293b', paddingTop: 16 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, color: '#10b981' }}>
            <CheckCircle2 size={14} />
            <span>Gateway Engine Active</span>
          </div>
          <div style={{ fontSize: 11, color: '#64748b', marginTop: 4 }}>Port: 8080 (Traffic) / 8081 (Admin)</div>
        </div>
      </aside>

      {/* Main Content Area */}
      <main style={{ flex: 1, padding: 32, overflowY: 'auto' }}>
        {/* Top Header */}
        <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 28 }}>
          <div>
            <h1 style={{ fontSize: 24, fontWeight: 700, margin: 0, color: '#f8fafc' }}>
              {tab === 'dashboard' && 'Platform Overview & Live Traffic'}
              {tab === 'routes' && 'Routing Rules & Policies'}
              {tab === 'upstreams' && 'Backend Upstreams'}
              {tab === 'keys' && 'Cryptographic API Keys'}
              {tab === 'ratelimits' && 'Rate Limiting Policies'}
              {tab === 'logs' && 'Live Traffic Inspection'}
              {tab === 'audit' && 'System Audit Trail'}
            </h1>
            <p style={{ margin: '4px 0 0', fontSize: 13, color: '#64748b' }}>
              Real-time HTTP traffic control, security policies, and edge routing
            </p>
          </div>

          <div style={{ display: 'flex', gap: 12 }}>
            <button 
              onClick={fetchData} 
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                padding: '8px 14px',
                borderRadius: 6,
                backgroundColor: '#1e293b',
                color: '#e2e8f0',
                border: '1px solid #334155',
                cursor: 'pointer',
                fontSize: 13
              }}
            >
              <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
              Sync
            </button>
          </div>
        </header>

        {/* TAB 1: DASHBOARD */}
        {tab === 'dashboard' && (
          <div>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 16, marginBottom: 24 }}>
              <div style={{ backgroundColor: '#0f1422', border: '1px solid #1e293b', borderRadius: 10, padding: 20 }}>
                <div style={{ fontSize: 12, color: '#94a3b8', fontWeight: 600 }}>TOTAL REQUESTS</div>
                <div style={{ fontSize: 28, fontWeight: 700, color: '#f8fafc', marginTop: 8 }}>{overview?.total_requests || 0}</div>
                <div style={{ fontSize: 12, color: '#38bdf8', marginTop: 4 }}>RPS: {overview?.rps?.toFixed(2) || '0.00'}</div>
              </div>

              <div style={{ backgroundColor: '#0f1422', border: '1px solid #1e293b', borderRadius: 10, padding: 20 }}>
                <div style={{ fontSize: 12, color: '#94a3b8', fontWeight: 600 }}>CACHE HIT RATIO</div>
                <div style={{ fontSize: 28, fontWeight: 700, color: '#10b981', marginTop: 8 }}>
                  {overview?.cache_hit_ratio ? `${(overview.cache_hit_ratio * 100).toFixed(1)}%` : '0%'}
                </div>
                <div style={{ fontSize: 12, color: '#64748b', marginTop: 4 }}>Hits: {overview?.cache_hits || 0} / Miss: {overview?.cache_misses || 0}</div>
              </div>

              <div style={{ backgroundColor: '#0f1422', border: '1px solid #1e293b', borderRadius: 10, padding: 20 }}>
                <div style={{ fontSize: 12, color: '#94a3b8', fontWeight: 600 }}>RATE LIMIT BLOCKS</div>
                <div style={{ fontSize: 28, fontWeight: 700, color: '#f59e0b', marginTop: 8 }}>{overview?.rate_limit_blocks || 0}</div>
                <div style={{ fontSize: 12, color: '#64748b', marginTop: 4 }}>HTTP 429 Throttle events</div>
              </div>

              <div style={{ backgroundColor: '#0f1422', border: '1px solid #1e293b', borderRadius: 10, padding: 20 }}>
                <div style={{ fontSize: 12, color: '#94a3b8', fontWeight: 600 }}>LATENCY (P95)</div>
                <div style={{ fontSize: 28, fontWeight: 700, color: '#a855f7', marginTop: 8 }}>
                  {overview?.p95_latency_ms ? `${overview.p95_latency_ms.toFixed(1)}ms` : '0.0ms'}
                </div>
                <div style={{ fontSize: 12, color: '#64748b', marginTop: 4 }}>P50: {overview?.p50_latency_ms?.toFixed(1) || 0}ms</div>
              </div>
            </div>

            {/* Quick Status Cards */}
            <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: 16 }}>
              <div style={{ backgroundColor: '#0f1422', border: '1px solid #1e293b', borderRadius: 10, padding: 20 }}>
                <h3 style={{ margin: '0 0 16px', fontSize: 15, fontWeight: 600 }}>Live Upstream Health</h3>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                  {upstreams.map(u => (
                    <div key={u.id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '12px 16px', backgroundColor: '#141a2c', borderRadius: 8 }}>
                      <div>
                        <div style={{ fontWeight: 600, fontSize: 14 }}>{u.name}</div>
                        <div style={{ fontSize: 12, color: '#64748b' }}>{u.base_url} ({u.algorithm})</div>
                      </div>
                      <span style={{ padding: '4px 8px', borderRadius: 4, fontSize: 11, fontWeight: 600, backgroundColor: u.healthy ? 'rgba(16, 185, 129, 0.15)' : 'rgba(239, 68, 68, 0.15)', color: u.healthy ? '#10b981' : '#ef4444' }}>
                        {u.healthy ? 'HEALTHY' : 'UNHEALTHY'}
                      </span>
                    </div>
                  ))}
                </div>
              </div>

              <div style={{ backgroundColor: '#0f1422', border: '1px solid #1e293b', borderRadius: 10, padding: 20 }}>
                <h3 style={{ margin: '0 0 16px', fontSize: 15, fontWeight: 600 }}>Security Status</h3>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 12, fontSize: 13 }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <span style={{ color: '#94a3b8' }}>SSRF Protection</span>
                    <span style={{ color: '#10b981', fontWeight: 600 }}>ENFORCED</span>
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <span style={{ color: '#94a3b8' }}>Cloudflare Trust Filter</span>
                    <span style={{ color: '#10b981', fontWeight: 600 }}>ACTIVE</span>
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <span style={{ color: '#94a3b8' }}>API Key Hashes</span>
                    <span style={{ color: '#38bdf8', fontWeight: 600 }}>SHA-256</span>
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <span style={{ color: '#94a3b8' }}>Rate Limiting Engine</span>
                    <span style={{ color: '#f59e0b', fontWeight: 600 }}>Token Bucket</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* TAB 2: ROUTES */}
        {tab === 'routes' && (
          <div style={{ backgroundColor: '#0f1422', border: '1px solid #1e293b', borderRadius: 10, padding: 20 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
              <h3 style={{ margin: 0, fontSize: 16 }}>Configured Routes</h3>
              <button 
                onClick={() => setNewRouteModal(true)} 
                style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '6px 12px', backgroundColor: '#0284c7', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontSize: 13, fontWeight: 600 }}
              >
                <Plus size={14} /> Add Route
              </button>
            </div>

            <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: 13 }}>
              <thead>
                <tr style={{ borderBottom: '1px solid #1e293b', color: '#94a3b8' }}>
                  <th style={{ padding: 12 }}>Name</th>
                  <th style={{ padding: 12 }}>Path Pattern</th>
                  <th style={{ padding: 12 }}>Methods</th>
                  <th style={{ padding: 12 }}>Auth Required</th>
                  <th style={{ padding: 12 }}>Target Upstream</th>
                  <th style={{ padding: 12 }}>Actions</th>
                </tr>
              </thead>
              <tbody>
                {routes.map(r => (
                  <tr key={r.id} style={{ borderBottom: '1px solid #141a2c' }}>
                    <td style={{ padding: 12, fontWeight: 600 }}>{r.name}</td>
                    <td style={{ padding: 12, fontFamily: 'monospace', color: '#38bdf8' }}>{r.path_pattern}</td>
                    <td style={{ padding: 12 }}>{r.methods.join(', ')}</td>
                    <td style={{ padding: 12 }}>
                      {r.auth_required ? (
                        <span style={{ color: '#10b981', display: 'flex', alignItems: 'center', gap: 4 }}><Lock size={14} /> Required</span>
                      ) : (
                        <span style={{ color: '#64748b' }}>Public</span>
                      )}
                    </td>
                    <td style={{ padding: 12 }}>{r.upstream_id}</td>
                    <td style={{ padding: 12 }}>
                      <button onClick={() => handleDeleteRoute(r.id)} style={{ background: 'none', border: 'none', color: '#ef4444', cursor: 'pointer' }}>
                        <Trash2 size={16} />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* TAB 3: API KEYS */}
        {tab === 'keys' && (
          <div style={{ backgroundColor: '#0f1422', border: '1px solid #1e293b', borderRadius: 10, padding: 20 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
              <h3 style={{ margin: 0, fontSize: 16 }}>Managed API Keys</h3>
              <button 
                onClick={() => { setNewKeyModal(true); setGeneratedKeyResult(null); }} 
                style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '6px 12px', backgroundColor: '#0284c7', color: '#fff', border: 'none', borderRadius: 6, cursor: 'pointer', fontSize: 13, fontWeight: 600 }}
              >
                <Plus size={14} /> Generate API Key
              </button>
            </div>

            <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: 13 }}>
              <thead>
                <tr style={{ borderBottom: '1px solid #1e293b', color: '#94a3b8' }}>
                  <th style={{ padding: 12 }}>Prefix</th>
                  <th style={{ padding: 12 }}>Owner</th>
                  <th style={{ padding: 12 }}>Description</th>
                  <th style={{ padding: 12 }}>Scopes</th>
                  <th style={{ padding: 12 }}>Status</th>
                  <th style={{ padding: 12 }}>Actions</th>
                </tr>
              </thead>
              <tbody>
                {apiKeys.map(k => (
                  <tr key={k.id} style={{ borderBottom: '1px solid #141a2c' }}>
                    <td style={{ padding: 12, fontFamily: 'monospace', color: '#38bdf8' }}>{k.key_prefix}••••••••</td>
                    <td style={{ padding: 12, fontWeight: 600 }}>{k.owner}</td>
                    <td style={{ padding: 12, color: '#94a3b8' }}>{k.description}</td>
                    <td style={{ padding: 12 }}>{k.scopes?.join(', ') || '*'}</td>
                    <td style={{ padding: 12 }}>
                      <span style={{ padding: '3px 8px', borderRadius: 4, fontSize: 11, fontWeight: 600, backgroundColor: k.active ? 'rgba(16, 185, 129, 0.15)' : 'rgba(239, 68, 68, 0.15)', color: k.active ? '#10b981' : '#ef4444' }}>
                        {k.active ? 'ACTIVE' : 'REVOKED'}
                      </span>
                    </td>
                    <td style={{ padding: 12 }}>
                      {k.active && (
                        <button onClick={() => handleRevokeKey(k.id)} style={{ background: 'none', border: 'none', color: '#ef4444', cursor: 'pointer', fontSize: 12, fontWeight: 600 }}>
                          Revoke
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* TAB: UPSTREAMS */}
        {tab === 'upstreams' && (
          <div style={{ backgroundColor: '#0f1422', border: '1px solid #1e293b', borderRadius: 10, padding: 20 }}>
            <h3 style={{ margin: '0 0 16px', fontSize: 16 }}>Registered Upstream Clusters</h3>
            <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: 13 }}>
              <thead>
                <tr style={{ borderBottom: '1px solid #1e293b', color: '#94a3b8' }}>
                  <th style={{ padding: 12 }}>Name</th>
                  <th style={{ padding: 12 }}>Target Base URL</th>
                  <th style={{ padding: 12 }}>Algorithm</th>
                  <th style={{ padding: 12 }}>Weight</th>
                  <th style={{ padding: 12 }}>Health Status</th>
                </tr>
              </thead>
              <tbody>
                {upstreams.map(u => (
                  <tr key={u.id} style={{ borderBottom: '1px solid #141a2c' }}>
                    <td style={{ padding: 12, fontWeight: 600 }}>{u.name}</td>
                    <td style={{ padding: 12, fontFamily: 'monospace', color: '#38bdf8' }}>{u.base_url}</td>
                    <td style={{ padding: 12 }}>{u.algorithm}</td>
                    <td style={{ padding: 12 }}>{u.weight}</td>
                    <td style={{ padding: 12 }}>
                      <span style={{ padding: '3px 8px', borderRadius: 4, fontSize: 11, fontWeight: 600, backgroundColor: u.healthy ? 'rgba(16, 185, 129, 0.15)' : 'rgba(239, 68, 68, 0.15)', color: u.healthy ? '#10b981' : '#ef4444' }}>
                        {u.healthy ? 'HEALTHY' : 'UNHEALTHY'}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* TAB: RATE LIMITS */}
        {tab === 'ratelimits' && (
          <div style={{ backgroundColor: '#0f1422', border: '1px solid #1e293b', borderRadius: 10, padding: 20 }}>
            <h3 style={{ margin: '0 0 16px', fontSize: 16 }}>Rate Limiting Policies</h3>
            <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: 13 }}>
              <thead>
                <tr style={{ borderBottom: '1px solid #1e293b', color: '#94a3b8' }}>
                  <th style={{ padding: 12 }}>Policy Name</th>
                  <th style={{ padding: 12 }}>Dimension</th>
                  <th style={{ padding: 12 }}>Request Limit</th>
                  <th style={{ padding: 12 }}>Window (sec)</th>
                  <th style={{ padding: 12 }}>Burst Capacity</th>
                </tr>
              </thead>
              <tbody>
                {rateLimits.map(p => (
                  <tr key={p.id} style={{ borderBottom: '1px solid #141a2c' }}>
                    <td style={{ padding: 12, fontWeight: 600 }}>{p.name}</td>
                    <td style={{ padding: 12 }}>{p.dimension}</td>
                    <td style={{ padding: 12, color: '#f59e0b', fontWeight: 600 }}>{p.limit} req</td>
                    <td style={{ padding: 12 }}>{p.window_sec}s</td>
                    <td style={{ padding: 12 }}>+{p.burst}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* TAB 4: LOGS */}
        {tab === 'logs' && (
          <div style={{ backgroundColor: '#0f1422', border: '1px solid #1e293b', borderRadius: 10, padding: 20 }}>
            <h3 style={{ margin: '0 0 16px', fontSize: 16 }}>Live Request Stream</h3>
            <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: 12 }}>
              <thead>
                <tr style={{ borderBottom: '1px solid #1e293b', color: '#94a3b8' }}>
                  <th style={{ padding: 10 }}>Time</th>
                  <th style={{ padding: 10 }}>Method</th>
                  <th style={{ padding: 10 }}>Path</th>
                  <th style={{ padding: 10 }}>Status</th>
                  <th style={{ padding: 10 }}>Duration</th>
                  <th style={{ padding: 10 }}>Cache</th>
                  <th style={{ padding: 10 }}>Client IP</th>
                </tr>
              </thead>
              <tbody>
                {requestLogs.map(l => (
                  <tr key={l.request_id} style={{ borderBottom: '1px solid #141a2c' }}>
                    <td style={{ padding: 10, color: '#64748b' }}>{new Date(l.timestamp).toLocaleTimeString()}</td>
                    <td style={{ padding: 10, fontWeight: 600 }}>{l.method}</td>
                    <td style={{ padding: 10, fontFamily: 'monospace' }}>{l.path}</td>
                    <td style={{ padding: 10, fontWeight: 700, color: l.status_code < 400 ? '#10b981' : (l.status_code === 429 ? '#f59e0b' : '#ef4444') }}>
                      {l.status_code}
                    </td>
                    <td style={{ padding: 10 }}>{(l.duration / 1000000).toFixed(2)}ms</td>
                    <td style={{ padding: 10 }}>
                      <span style={{ color: l.cache_status === 'HIT' ? '#10b981' : '#64748b', fontWeight: 600 }}>{l.cache_status}</span>
                    </td>
                    <td style={{ padding: 10, fontFamily: 'monospace', color: '#94a3b8' }}>{l.client_ip}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* TAB 5: AUDIT */}
        {tab === 'audit' && (
          <div style={{ backgroundColor: '#0f1422', border: '1px solid #1e293b', borderRadius: 10, padding: 20 }}>
            <h3 style={{ margin: '0 0 16px', fontSize: 16 }}>Audit Trail</h3>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
              {auditLogs.map(a => (
                <div key={a.id} style={{ padding: 14, backgroundColor: '#141a2c', borderRadius: 8, display: 'flex', justifyContent: 'space-between', alignItems: 'center', fontSize: 13 }}>
                  <div>
                    <span style={{ fontWeight: 600, color: '#38bdf8' }}>[{a.action}]</span> {a.entity_type} ({a.entity_id})
                    <div style={{ fontSize: 11, color: '#64748b', marginTop: 2 }}>Admin: {a.admin_user} • IP: {a.ip_address}</div>
                  </div>
                  <div style={{ fontSize: 11, color: '#94a3b8' }}>{new Date(a.timestamp).toLocaleString()}</div>
                </div>
              ))}
            </div>
          </div>
        )}
      </main>

      {/* Modal: New API Key */}
      {newKeyModal && (
        <div style={{ position: 'fixed', inset: 0, backgroundColor: 'rgba(0,0,0,0.7)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 100 }}>
          <div style={{ backgroundColor: '#0f1422', border: '1px solid #334155', borderRadius: 12, padding: 28, width: 460 }}>
            <h2 style={{ margin: '0 0 16px', fontSize: 18 }}>Generate Secure API Key</h2>
            {generatedKeyResult ? (
              <div>
                <div style={{ padding: 12, backgroundColor: 'rgba(16, 185, 129, 0.1)', border: '1px solid #10b981', borderRadius: 6, color: '#10b981', fontSize: 13, marginBottom: 16 }}>
                  Key generated successfully! Copy it now; it will <strong>never</strong> be shown again.
                </div>
                <div style={{ padding: 12, backgroundColor: '#05070a', borderRadius: 6, fontFamily: 'monospace', fontSize: 14, color: '#38bdf8', wordBreak: 'break-all', marginBottom: 20 }}>
                  {generatedKeyResult}
                </div>
                <button onClick={() => { setNewKeyModal(false); setGeneratedKeyResult(null); }} style={{ width: '100%', padding: 10, backgroundColor: '#0284c7', color: '#fff', border: 'none', borderRadius: 6, fontWeight: 600, cursor: 'pointer' }}>
                  Done
                </button>
              </div>
            ) : (
              <form onSubmit={handleCreateKey} style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
                <div>
                  <label style={{ fontSize: 12, color: '#94a3b8', display: 'block', marginBottom: 6 }}>Key Owner / Client</label>
                  <input required value={keyOwner} onChange={e => setKeyOwner(e.target.value)} placeholder="e.g. MobileApp" style={{ width: '100%', padding: 10, borderRadius: 6, backgroundColor: '#141a2c', border: '1px solid #334155', color: '#fff' }} />
                </div>
                <div>
                  <label style={{ fontSize: 12, color: '#94a3b8', display: 'block', marginBottom: 6 }}>Description</label>
                  <input value={keyDesc} onChange={e => setKeyDesc(e.target.value)} placeholder="e.g. Production iOS client" style={{ width: '100%', padding: 10, borderRadius: 6, backgroundColor: '#141a2c', border: '1px solid #334155', color: '#fff' }} />
                </div>
                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10, marginTop: 10 }}>
                  <button type="button" onClick={() => setNewKeyModal(false)} style={{ padding: '8px 14px', backgroundColor: 'transparent', color: '#94a3b8', border: 'none', cursor: 'pointer' }}>Cancel</button>
                  <button type="submit" style={{ padding: '8px 16px', backgroundColor: '#0284c7', color: '#fff', border: 'none', borderRadius: 6, fontWeight: 600, cursor: 'pointer' }}>Generate</button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}

      {/* Modal: New Route */}
      {newRouteModal && (
        <div style={{ position: 'fixed', inset: 0, backgroundColor: 'rgba(0,0,0,0.7)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 100 }}>
          <div style={{ backgroundColor: '#0f1422', border: '1px solid #334155', borderRadius: 12, padding: 28, width: 460 }}>
            <h2 style={{ margin: '0 0 16px', fontSize: 18 }}>Add Routing Rule</h2>
            <form onSubmit={handleCreateRoute} style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
              <div>
                <label style={{ fontSize: 12, color: '#94a3b8', display: 'block', marginBottom: 6 }}>Route Name</label>
                <input required value={routeName} onChange={e => setRouteName(e.target.value)} placeholder="e.g. Users API" style={{ width: '100%', padding: 10, borderRadius: 6, backgroundColor: '#141a2c', border: '1px solid #334155', color: '#fff' }} />
              </div>
              <div>
                <label style={{ fontSize: 12, color: '#94a3b8', display: 'block', marginBottom: 6 }}>Path Pattern</label>
                <input required value={routePath} onChange={e => setRoutePath(e.target.value)} placeholder="e.g. /users/*" style={{ width: '100%', padding: 10, borderRadius: 6, backgroundColor: '#141a2c', border: '1px solid #334155', color: '#fff' }} />
              </div>
              <div>
                <label style={{ fontSize: 12, color: '#94a3b8', display: 'block', marginBottom: 6 }}>Target Upstream</label>
                <select value={routeUpstream} onChange={e => setRouteUpstream(e.target.value)} style={{ width: '100%', padding: 10, borderRadius: 6, backgroundColor: '#141a2c', border: '1px solid #334155', color: '#fff' }}>
                  {upstreams.map(u => (
                    <option key={u.id} value={u.id}>{u.name} ({u.base_url})</option>
                  ))}
                </select>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <input type="checkbox" id="authCb" checked={routeAuth} onChange={e => setRouteAuth(e.target.checked)} />
                <label htmlFor="authCb" style={{ fontSize: 13, color: '#e2e8f0' }}>Require API Key Authentication</label>
              </div>
              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10, marginTop: 10 }}>
                <button type="button" onClick={() => setNewRouteModal(false)} style={{ padding: '8px 14px', backgroundColor: 'transparent', color: '#94a3b8', border: 'none', cursor: 'pointer' }}>Cancel</button>
                <button type="submit" style={{ padding: '8px 16px', backgroundColor: '#0284c7', color: '#fff', border: 'none', borderRadius: 6, fontWeight: 600, cursor: 'pointer' }}>Create Route</button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
