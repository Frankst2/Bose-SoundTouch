async function req(url, opts = {}) {
    const r = await fetch(url, opts);
    return r.json();
}

export const api = {
    devices: () => req('/api/devices'),
    device: (id) => req(`/api/device/${id}`),
    discover: () => req('/api/discover', { method: 'POST' }),
    key: (id, key) => req(`/api/device-key/${id}/${key}`, { method: 'POST' }),
    volume: (id, level) => req(`/api/device-volume/${id}/${level}`, { method: 'POST' }),
    power: (id) => req(`/api/device-power/${id}`, { method: 'POST' }),
    tuneInBrowse: (path) => req(path ? `/api/tunein/navigate/${path}` : '/api/tunein/navigate'),
    tuneInSearch: (q) => req(`/api/tunein/search?q=${encodeURIComponent(q)}`),
    control: (id, action, presetId) => req(`/api/control/${id}/${action}?id=${presetId}`),
    selectSource: (id, source, account) => req(`/api/control/${id}/source?name=${encodeURIComponent(source)}&account=${encodeURIComponent(account || '')}`),
    tuneInPlay: (deviceId, item) => req(`/api/tunein/play/${deviceId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(item),
    }),
};