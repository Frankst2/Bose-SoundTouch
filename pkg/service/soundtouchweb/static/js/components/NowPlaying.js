import { h } from 'preact';
import htm from 'htm';

const html = htm.bind(h);

export function NowPlaying({ nowPlaying }) {
    if (!nowPlaying || nowPlaying.Source === 'STANDBY') {
        return html`<div class="now-playing standby">Standby</div>`;
    }

    const title = nowPlaying.Track || nowPlaying.StationName || nowPlaying.Source;
    const artURL = nowPlaying.Art?.URL;
    const isPlaying = nowPlaying.PlayStatus === 'PLAY_STATE';
    const isBuffering = nowPlaying.PlayStatus === 'BUFFERING_STATE';

    return html`
        <div class="now-playing">
            ${artURL && html`
                <img class="album-art" src=${artURL} alt="" />
            `}
            <div class="track-info">
                <div class="track-title">${title}</div>
                ${nowPlaying.Artist && html`<div class="track-artist">${nowPlaying.Artist}</div>`}
                ${nowPlaying.Album && html`<div class="track-album">${nowPlaying.Album}</div>`}
                <div class="track-meta">
                    <span class="track-source">${nowPlaying.Source}</span>
                    ${isBuffering && html`<span class="buffering-badge">Buffering…</span>`}
                </div>
            </div>
        </div>
    `;
}