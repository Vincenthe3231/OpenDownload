export const useDownloadEngine = () => {
    const startDownload = async (url: string, path: string): Promise<void> => {
        try {
            // Wails binds Go methods to window.go
            await (window as any).go.main.App.Download(url, path);
            console.log('Backend Download initiated:', url);
        } catch (e) {
            console.error('Download failed:', e);
        }
    };

    return { startDownload };
};
