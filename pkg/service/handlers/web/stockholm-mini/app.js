async function fetchDevices() {
    try {
        const response = await fetch('/devices');
        const devices = await response.json();
        const container = document.getElementById('device-list');
        const seen = new Set();

        if (devices.length === 0) {
            container.innerHTML = '<p>No devices found. Ensure they are on the same network.</p>';
            return;
        }

        devices.forEach(device => {
            seen.add(device.device_id);
            const existing = document.getElementById(`device-${device.device_id}`);
            if (existing) {
                // Update product code/IP if changed, but keep title if we already have a better name
                const title = existing.querySelector('.device-title');
                if (title && (!title.textContent || title.textContent === 'Unknown Device' || title.textContent.startsWith('SoundTouch-'))) {
                    title.textContent = device.name || 'Unknown Device';
                }
                const subtitle = existing.querySelector('.device-subtitle span');
                if (subtitle) {
                    const currentSubtitle = subtitle.textContent || '';
                    const parts = currentSubtitle.split(' | ');
                    const currentType = parts.length > 1 ? parts[1].trim() : '';
                    const newType = device.product_code || 'Unknown';

                    // Don't downgrade type if we already have a specific one
                    const isGeneric = !currentType || currentType === 'Unknown' || currentType === 'N/A';
                    const displayType = isGeneric ? newType : currentType;
                    subtitle.textContent = `${device.ip_address} | ${displayType}`;
                }
                const details = existing.querySelector(`#details-${device.device_id}`);
                if (details) {
                    const idField = details.querySelector('p:nth-child(1) code');
                    if (idField) {
                        const currentId = idField.textContent;
                        // Don't overwrite with serial if we have a real deviceID (usually hex)
                        if (!currentId || currentId === 'N/A' || currentId === device.device_serial_number) {
                            idField.textContent = device.device_id || 'N/A';
                        }
                    }
                    const firmwareField = details.querySelector('p:nth-child(2) code');
                    if (firmwareField) {
                        const cur = firmwareField.textContent;
                        if (!cur || cur === 'N/A' || cur === '0.0.0') {
                            firmwareField.textContent = device.firmware_version || 'N/A';
                        }
                    }
                    const serialField = details.querySelector('p:nth-child(3) code');
                    if (serialField && (!serialField.textContent || serialField.textContent === 'N/A')) {
                        serialField.textContent = device.device_serial_number || 'N/A';
                    }
                }
                // Ensure WS is open
                openDeviceWebSocket(device.device_id);
                return;
            }

            const card = document.createElement('div');
            card.className = 'device-card';
            card.id = `device-${device.device_id}`;
            card.innerHTML = `
                <div class="device-info">
                    <div class="device-header">
                        <div>
                            <div class="device-title-row">
                                <h2 class="device-title">${device.name || 'Unknown Device'}</h2>
                                <button class="info-toggle" title="More info" onclick="toggleDetails('${device.device_id}')">i</button>
                            </div>
                            <p class="device-subtitle">
                                <span>${device.ip_address} | ${device.product_code}</span>
                            </p>
                        </div>
                        <button class="power-icon" title="Power" aria-label="Power" onclick="control('${device.device_id}', 'POWER')">&#xE17E;</button>
                    </div>
                    <div class="device-details" id="details-${device.device_id}">
                        <p>ID: <code>${device.device_id}</code></p>
                        <p>Firmware: <code>${device.firmware_version || 'N/A'}</code></p>
                        <p>Serial: <code>${device.device_serial_number || 'N/A'}</code></p>
                        <p>Discovery: <code>${device.discovery_method || 'N/A'}</code></p>
                    </div>
                </div>
                <div class="now-playing" id="np-${device.device_id}">
                    <p><em>Loading playback status...</em></p>
                </div>
                <div class="controls">
                    <button class="primary" onclick="control('${device.device_id}', 'PLAY')">Play</button>
                    <button class="primary" onclick="control('${device.device_id}', 'PAUSE')">Pause</button>
                    <button onclick="control('${device.device_id}', 'PREV_TRACK')">Prev</button>
                    <button onclick="control('${device.device_id}', 'NEXT_TRACK')">Next</button>
                </div>
                <div class="volume-container">
                    <span>Vol:</span>
                    <input id="vol-${device.device_id}" type="range" min="0" max="100"
                        oninput="onVolumeInput('${device.device_id}', this)"
                        onmousedown="startAdjust('${device.device_id}')" ontouchstart="startAdjust('${device.device_id}')"
                        onmouseup="endAdjust('${device.device_id}')" ontouchend="endAdjust('${device.device_id}')">
                </div>
            `;
            container.appendChild(card);
            updateNowPlaying(device.device_id);
            updateVolume(device.device_id);
            openDeviceWebSocket(device.device_id);
        });

        // Remove cards for devices that no longer exist
        Array.from(container.children).forEach(child => {
            const id = child.id?.replace('device-', '');
            if (id && !seen.has(id)) {
                container.removeChild(child);
            }
        });
    } catch (error) {
        console.error('Failed to fetch devices', error);
        document.getElementById('device-list').innerHTML = '<p>Error loading devices.</p>';
    }
}

async function updateNowPlaying(deviceId) {
    try {
        const response = await fetch(`/devices/${deviceId}/info`);
        if (!response.ok) return;
        const info = await response.json();

        // Update device name and type if available (live info is more accurate than discovery)
        const title = document.querySelector(`#device-${deviceId} .device-title`);
        if (title && info.name) {
            title.textContent = info.name;
        }
        const subtitle = document.querySelector(`#device-${deviceId} .device-subtitle span`);
        if (subtitle && info.type) {
            subtitle.textContent = `${info.ipAddress || info.ip_address || 'N/A'} | ${info.type}`;
        }

        // Update firmware version if available
        const details = document.getElementById(`details-${deviceId}`);
        if (details) {
            if (info.deviceID) {
                const idField = details.querySelector('p:nth-child(1) code');
                if (idField) idField.textContent = info.deviceID;
            }
            if (info.softwareVersion) {
                const firmwareField = details.querySelector('p:nth-child(2) code');
                if (firmwareField) firmwareField.textContent = info.softwareVersion;
            }
            if (info.serialNumber) {
                const serialField = details.querySelector('p:nth-child(3) code');
                if (serialField) serialField.textContent = info.serialNumber;
            }
        }

        const npContainer = document.getElementById(`np-${deviceId}`);
        if (npContainer && info.nowPlaying) {
            const np = info.nowPlaying;
            const source = np.source || np.Source;
            const powerIcon = document.querySelector(`#device-${deviceId} .power-icon`);
            if (powerIcon) {
                if (source === 'STANDBY') {
                    powerIcon.classList.add('off');
                    powerIcon.classList.remove('on');
                } else {
                    powerIcon.classList.remove('off');
                    powerIcon.classList.add('on');
                }
            }
            if (source === 'STANDBY') {
                npContainer.innerHTML = '<div class="now-playing-info"><p><em>Standby</em></p></div>';
            } else {
                const track = np.track || np.Track || np.stationName || np.StationName || 'Unknown Track';
                const artist = np.artist || np.Artist || 'Unknown Artist';
                const album = np.album || np.Album || 'Unknown Album';
                const art = np.Art || np.art || {};
                const artStatus = art.ArtImageStatus || art.artImageStatus;
                const artUrl = artStatus === 'IMAGE_PRESENT' ? (art.URL || art.url || '') : '';

                npContainer.innerHTML = `
                    <img class="album-art" src="${artUrl}" alt="Artwork">
                    <div class="now-playing-info">
                        <strong>${track}</strong><br>
                        ${artist} - ${album}
                    </div>
                `;
            }
        }
    } catch (error) {
        console.warn('Failed to fetch now playing for ' + deviceId, error);
    }
}

async function updateVolume(deviceId) {
    try {
        const response = await fetch(`/devices/${deviceId}/info`);
        if (!response.ok) return;
        const info = await response.json();
        const slider = document.getElementById(`vol-${deviceId}`);
        if (slider && info.volume && typeof info.volume.actualvolume === 'number' && !adjusting[deviceId]) {
            slider.value = String(info.volume.actualvolume);
        }
    } catch (error) {
        console.warn('Failed to fetch volume for ' + deviceId, error);
    }
}

async function control(deviceId, key) {
    let deviceName = deviceId;
    const title = document.querySelector(`#device-${deviceId} .device-title`);
    if (title && title.textContent) {
        deviceName = title.textContent;
    }

    try {
        const res = await fetch(`/devices/${encodeURIComponent(deviceId)}/key/${encodeURIComponent(key)}`, {
            method: 'POST'
        });
        if (!res.ok) {
            const text = await res.text();
            throw new Error(text || `HTTP ${res.status}`);
        }
    } catch (error) {
        console.error('Control failed', error);
        alert(`Failed to send ${key} to ${deviceName}: ${error.message}`);
    }
}

async function setVolume(deviceId, level) {
    let deviceName = deviceId;
    const title = document.querySelector(`#device-${deviceId} .device-title`);
    if (title && title.textContent) {
        deviceName = title.textContent;
    }

    try {
        const res = await fetch(`/devices/${encodeURIComponent(deviceId)}/volume/${encodeURIComponent(level)}`, {
            method: 'POST'
        });
        if (!res.ok) {
            const text = await res.text();
            throw new Error(text || `HTTP ${res.status}`);
        }
    } catch (error) {
        console.error('Set volume failed', error);
        alert(`Failed to set volume on ${deviceName} to ${level}: ${error.message}`);
    }
}

// Volume interaction helpers to avoid UI jumping while dragging
const adjusting = {};
const volumeTimers = {};

function startAdjust(deviceId) {
    adjusting[deviceId] = true;
}

function endAdjust(deviceId) {
    // Small delay to let the device send back its volume update
    setTimeout(() => { adjusting[deviceId] = false; }, 300);
}

function onVolumeInput(deviceId, el) {
    startAdjust(deviceId);
    const level = el.value;
    // Debounce network calls per device
    if (volumeTimers[deviceId]) {
        clearTimeout(volumeTimers[deviceId]);
    }
    volumeTimers[deviceId] = setTimeout(() => {
        setVolume(deviceId, level);
        endAdjust(deviceId);
    }, 150);
}

let deviceSockets = {};

function openDeviceWebSocket(deviceId) {
    const key = `${deviceId}`;
    try {
        const existing = deviceSockets[key];
        if (existing) {
            // Reuse an already healthy connection instead of tearing it down every refresh
            if (existing.readyState === WebSocket.OPEN || existing.readyState === WebSocket.CONNECTING) {
                return;
            }
            try { existing.close(); } catch (_) {}
        }
        const proto = location.protocol === 'https:' ? 'wss' : 'ws';
        const wsUrl = `${proto}://${location.host}/devices/${encodeURIComponent(deviceId)}/ws`;
        const ws = new WebSocket(wsUrl);
        deviceSockets[key] = ws;

        ws.onopen = () => {
            // console.log('WS connected for', deviceId);
        };
        ws.onmessage = (ev) => {
            try {
                const msg = JSON.parse(ev.data);
                const type = msg.type;
                const payload = msg.payload || {};
                if (type === 'nowPlayingUpdated') {
                    const e = payload;
                    const np = e.NowPlaying || e.nowPlaying || {};
                    const source = np.source || np.Source;

                    // Also try to update name/type if they are present in the event (sometimes events carry device info)
                    const title = document.querySelector(`#device-${deviceId} .device-title`);
                    if (title && e.name) {
                        title.textContent = e.name;
                    }
                    const subtitle = document.querySelector(`#device-${deviceId} .device-subtitle span`);
                    if (subtitle && e.type) {
                        subtitle.textContent = `${e.ipAddress || e.ip_address || 'N/A'} | ${e.type}`;
                    }

                    const powerIcon = document.querySelector(`#device-${deviceId} .power-icon`);
                    if (powerIcon) {
                        if (source === 'STANDBY') {
                            powerIcon.classList.add('off');
                            powerIcon.classList.remove('on');
                        } else {
                            powerIcon.classList.remove('off');
                            powerIcon.classList.add('on');
                        }
                    }
                    const npContainer = document.getElementById(`np-${deviceId}`);
                    if (npContainer) {
                        if (source === 'STANDBY') {
                            npContainer.innerHTML = '<div class="now-playing-info"><p><em>Standby</em></p></div>';
                        } else {
                            const track = np.track || np.Track || np.stationName || np.StationName || 'Unknown Track';
                            const artist = np.artist || np.Artist || 'Unknown Artist';
                            const album = np.album || np.Album || 'Unknown Album';
                            const art = np.Art || np.art || {};
                            const artStatus = art.ArtImageStatus || art.artImageStatus;
                            const artUrl = artStatus === 'IMAGE_PRESENT' ? (art.URL || art.url || '') : '';

                            npContainer.innerHTML = `
                                <img class="album-art" src="${artUrl}" alt="Artwork">
                                <div class="now-playing-info">
                                    <strong>${track}</strong><br>
                                    ${artist} - ${album}
                                </div>
                            `;
                        }
                    }
                } else if (type === 'volumeUpdated') {
                    const e = payload;
                    const vol = (e.Volume && (typeof e.Volume.actualvolume === 'number' ? e.Volume.actualvolume : (typeof e.Volume.actual === 'number' ? e.Volume.actual : e.Volume.target))) ||
                                (e.volume && (typeof e.volume.actualvolume === 'number' ? e.volume.actualvolume : (typeof e.volume.actual === 'number' ? e.volume.actual : e.volume.target)));
                    const slider = document.getElementById(`vol-${deviceId}`);
                    if (slider && typeof vol === 'number' && !adjusting[deviceId]) {
                        slider.value = String(vol);
                    }
                } else if (type === 'snapshotInfo') {
                    const info = payload || {};

                    // Update name and type from snapshot
                    const title = document.querySelector(`#device-${deviceId} .device-title`);
                    if (title && info.name) {
                        title.textContent = info.name;
                    }
                    const subtitle = document.querySelector(`#device-${deviceId} .device-subtitle span`);
                    if (subtitle && info.type) {
                        subtitle.textContent = `${info.ipAddress || info.ip_address || 'N/A'} | ${info.type}`;
                    }

                    // Update firmware and ID from snapshot if available
                    const details = document.getElementById(`details-${deviceId}`);
                    if (details) {
                        if (info.deviceID) {
                            const idField = details.querySelector('p:nth-child(1) code');
                            if (idField) idField.textContent = info.deviceID;
                        }
                        if (info.softwareVersion) {
                            const firmwareField = details.querySelector('p:nth-child(2) code');
                            if (firmwareField) firmwareField.textContent = info.softwareVersion;
                        }
                        if (info.serialNumber) {
                            const serialField = details.querySelector('p:nth-child(3) code');
                            if (serialField) serialField.textContent = info.serialNumber;
                        }
                    }

                    if (info.nowPlaying) {
                        const np = info.nowPlaying;
                        const source = np.source || np.Source;
                        const powerIcon = document.querySelector(`#device-${deviceId} .power-icon`);
                        if (powerIcon) {
                            if (source === 'STANDBY') {
                                powerIcon.classList.add('off');
                                powerIcon.classList.remove('on');
                            } else {
                                powerIcon.classList.remove('off');
                                powerIcon.classList.add('on');
                            }
                        }
                        const npContainer = document.getElementById(`np-${deviceId}`);
                        if (npContainer) {
                            if (source === 'STANDBY') {
                                npContainer.innerHTML = '<div class="now-playing-info"><p><em>Standby</em></p></div>';
                            } else {
                                const track = np.track || np.Track || np.stationName || np.StationName || 'Unknown Track';
                                const artist = np.artist || np.Artist || 'Unknown Artist';
                                const album = np.album || np.Album || 'Unknown Album';
                                const art = np.Art || np.art || {};
                                const artStatus = art.ArtImageStatus || art.artImageStatus;
                                const artUrl = artStatus === 'IMAGE_PRESENT' ? (art.URL || art.url || '') : '';

                                npContainer.innerHTML = `
                                    <img class="album-art" src="${artUrl}" alt="Artwork">
                                    <div class="now-playing-info">
                                        <strong>${track}</strong><br>
                                        ${artist} - ${album}
                                    </div>
                                `;
                            }
                        }
                    }
                    const vol = info.actualVolume || (info.volume && (typeof info.volume.actualvolume === 'number' ? info.volume.actualvolume : (typeof info.volume.actual === 'number' ? info.volume.actual : null)));
                    const slider = document.getElementById(`vol-${deviceId}`);
                    if (slider && typeof vol === 'number' && !adjusting[deviceId]) slider.value = String(vol);
                }
            } catch (err) {
                // console.warn('Bad WS message', err);
            }
        };
        ws.onerror = () => {
            // console.warn('WS error for', ip);
        };
        ws.onclose = () => {
            // Try to reconnect after a delay
            setTimeout(() => {
                if (deviceSockets[key] === ws) {
                    delete deviceSockets[key];
                }
                openDeviceWebSocket(deviceId);
            }, 3000);
        };
    } catch (e) {
        // console.warn('Failed to open WS for', deviceId, e);
    }
}

function toggleDetails(deviceId) {
    const el = document.getElementById(`details-${deviceId}`);
    if (el) {
        el.classList.toggle('visible');
    }
}


document.addEventListener('DOMContentLoaded', () => {
    fetchDevices();
    fetchVersion();
    setInterval(fetchDevices, 30000);
});
