async function fetchVersion() {
    try {
        const response = await fetch('/version');
        const data = await response.json();
        const info = document.getElementById('version-info');
        if (info && data.version) {
            const version = data.version;
            const commit = data.commit;
            const isDirty = version.includes('dirty');
            const releaseUrl = isDirty
                ? 'https://github.com/gesellix/Bose-SoundTouch/releases'
                : `https://github.com/gesellix/Bose-SoundTouch/releases/tag/v${version}`;
            const commitUrl = `https://github.com/gesellix/Bose-SoundTouch/commit/${commit}`;
            const projectUrl = 'https://gesellix.github.io/Bose-SoundTouch/';

            info.innerHTML = `<a href="${projectUrl}" target="_blank" style="color: inherit; text-decoration: none;">AfterTouch</a> ` +
                           `<a href="${releaseUrl}" target="_blank" style="color: inherit;">${version}</a> ` +
                           `(<a href="${commitUrl}" target="_blank" style="color: inherit;">${commit}</a>) - ${data.date}`;
        }
    } catch (error) {
        console.error('Failed to fetch version info', error);
    }
}
