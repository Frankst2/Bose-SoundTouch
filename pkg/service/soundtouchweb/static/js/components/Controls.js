import { h } from 'preact';
import { useState, useEffect } from 'preact/hooks';
import htm from 'htm';
import { api } from '../api.js';

const html = htm.bind(h);

export function Controls({ deviceId, status }) {
    const np = status?.nowPlaying;
    const isPlaying = np?.PlayStatus === 'PLAY_STATE';
    const actualVolume = status?.volume?.ActualVolume ?? 0;
    const isMuted = status?.volume?.MuteEnabled ?? false;
    const skipEnabled = np?.SkipEnabled != null;
    const skipPrevEnabled = np?.SkipPreviousEnabled != null;

    const [localVolume, setLocalVolume] = useState(actualVolume);

    useEffect(() => { setLocalVolume(actualVolume); }, [actualVolume]);

    const send = (key) => api.key(deviceId, key);

    function onVolumeChange(e) {
        const val = parseInt(e.target.value, 10);
        setLocalVolume(val);
        api.volume(deviceId, val);
    }

    return html`
        <div class="controls">
            <div class="transport">
                <button class="ctrl-btn" onClick=${() => send('PREV_TRACK')} title="Previous">⏮</button>
                <button class="ctrl-btn play-btn" onClick=${() => send(isPlaying ? 'PAUSE' : 'PLAY')}>
                    ${isPlaying ? '⏸' : '▶'}
                </button>
                <button class="ctrl-btn" onClick=${() => send('NEXT_TRACK')} title="Next">⏭</button>
                <button class="ctrl-btn ${isMuted ? 'active' : ''}" onClick=${() => send('MUTE')} title="Mute">
                    ${isMuted ? '🔇' : '🔊'}
                </button>
            </div>
            <div class="volume-row">
                <span class="volume-icon">🔈</span>
                <input
                    type="range"
                    class="volume-slider"
                    min="0"
                    max="100"
                    value=${localVolume}
                    onInput=${onVolumeChange}
                />
                <span class="volume-value">${localVolume}</span>
            </div>
        </div>
    `;
}