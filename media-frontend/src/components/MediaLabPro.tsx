import React, { useState, useEffect, useRef } from 'react';
import { AdvancedUploader, type UploadProgressInfo } from '../upload/advancedUploader';
import { mediaSocket } from '../upload/websocket';
import { getDownloadUrl, deleteMedia } from '../api/operations';
import { formatBytes } from '../upload/useFileUpload';
import { downloadWithResume } from '../upload/resumableDownload';

interface QueueItem {
  id: string;
  file: File;
  uploader: AdvancedUploader;
  progress: UploadProgressInfo;
  mediaId: string | null;
  processingProgress: number | null;
  processingStatus: string | null;
  downloading: boolean;
  downloadProgress: number;
  downloadUrl: string | null;
  copyingLink: boolean;
}

export function MediaLabPro({ accessToken }: { accessToken: string | null }) {
  const [queue, setQueue] = useState<QueueItem[]>([]);
  const [dragActive, setDragActive] = useState(false);
  const queueRef = useRef<QueueItem[]>([]);
  
  useEffect(() => {
    queueRef.current = queue;
  }, [queue]);

  // Connect WebSocket on component mount
  useEffect(() => {
    if (accessToken) {
      mediaSocket.connect(accessToken);
      const unsubscribe = mediaSocket.onEvent((event) => {
        console.log('WS Event received:', event);
        if (event.mediaId) {
          setQueue((prev) =>
            prev.map((item) => {
              if (item.mediaId === event.mediaId) {
                let status = event.event.replace('_STARTED', '').replace('_COMPLETED', '');
                let prog = event.progress ?? null;
                if (event.event === 'READY') {
                  status = 'READY';
                  prog = 100;
                }
                return {
                  ...item,
                  processingStatus: status,
                  processingProgress: prog,
                };
              }
              return item;
            })
          );
        }
      });
      return () => {
        unsubscribe();
        mediaSocket.disconnect();
      };
    }
  }, [accessToken]);

  const handleDrag = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === 'dragenter' || e.type === 'dragover') {
      setDragActive(true);
    } else if (e.type === 'dragleave') {
      setDragActive(false);
    }
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragActive(false);
    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      addFilesToQueue(Array.from(e.dataTransfer.files));
    }
  };

  const handleFileInput = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files) {
      addFilesToQueue(Array.from(e.target.files));
    }
  };

  const addFilesToQueue = (files: File[]) => {
    const newItems = files.map((file) => {
      const id = `${file.name}-${file.size}-${Date.now()}`;
      
      const onProgressUpdate = (info: UploadProgressInfo) => {
        setQueue((prev) =>
          prev.map((item) => {
            if (item.id === id) {
              return {
                ...item,
                progress: info,
                mediaId: item.uploader.getMediaId() || item.mediaId,
              };
            }
            return item;
          })
        );
      };

      const uploader = new AdvancedUploader(file, onProgressUpdate);

      return {
        id,
        file,
        uploader,
        progress: {
          phase: 'IDLE' as const,
          uploadedBytes: 0,
          totalBytes: file.size,
          speedBytesPerSec: 0,
          parts: [],
        },
        mediaId: null,
        processingProgress: null,
        processingStatus: null,
        downloading: false,
        downloadProgress: 0,
        downloadUrl: null,
        copyingLink: false,
      };
    });

    setQueue((prev) => [...prev, ...newItems]);
  };

  const startUpload = (id: string) => {
    const item = queue.find((i) => i.id === id);
    if (item) {
      item.uploader.start();
    }
  };

  const pauseUpload = (id: string) => {
    const item = queue.find((i) => i.id === id);
    if (item) {
      item.uploader.pause();
    }
  };

  const resumeUpload = (id: string) => {
    const item = queue.find((i) => i.id === id);
    if (item) {
      item.uploader.resume();
    }
  };

  const cancelUpload = (id: string) => {
    const item = queue.find((i) => i.id === id);
    if (item) {
      item.uploader.cancel();
      setQueue((prev) => prev.filter((i) => i.id !== id));
    }
  };

  const downloadFile = async (item: QueueItem) => {
    if (!item.mediaId) return;
    setQueue(prev => prev.map(i => i.id === item.id ? { ...i, downloading: true, downloadProgress: 0 } : i));

    try {
      const dl = await getDownloadUrl(item.mediaId);
      await downloadWithResume({
        url: dl.url,
        fileName: item.file.name,
        contentType: dl.contentType,
        onProgress: (received, total) => {
          if (total && total > 0) {
            const progress = Math.min(100, Math.round((received / total) * 100));
            setQueue(prev => prev.map(i => i.id === item.id ? { ...i, downloadProgress: progress } : i));
          }
        },
      });
    } catch (err) {
      alert(`فشل تحميل الملف: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      setQueue(prev => prev.map(i => i.id === item.id ? { ...i, downloading: false } : i));
    }
  };

  const copyLink = async (item: QueueItem) => {
    if (!item.mediaId) return;
    setQueue(prev => prev.map(i => i.id === item.id ? { ...i, copyingLink: true } : i));
    try {
      const dl = await getDownloadUrl(item.mediaId);
      setQueue(prev => prev.map(i => i.id === item.id ? { ...i, downloadUrl: dl.url } : i));
      await navigator.clipboard.writeText(dl.url);
    } catch (err) {
      alert(`فشل نسخ الرابط: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      setQueue(prev => prev.map(i => i.id === item.id ? { ...i, copyingLink: false } : i));
    }
  };

  const deleteItem = async (item: QueueItem) => {
    if (!item.mediaId) return;
    setQueue(prev => prev.map(i => i.id === item.id ? { ...i, processingStatus: 'DELETING' } : i));
    
    try {
      await deleteMedia(item.mediaId);
      // Wait for WS DELETE_COMPLETED which will remove or mark it deleted.
    } catch (err) {
      alert(`فشل الحذف: ${err instanceof Error ? err.message : String(err)}`);
      setQueue(prev => prev.map(i => i.id === item.id ? { ...i, processingStatus: null } : i));
    }
  };

  const getPreviewElement = (file: File) => {
    const url = URL.createObjectURL(file);
    if (file.type.startsWith('image/')) {
      return <img src={url} alt="Preview" style={{ width: 80, height: 80, objectFit: 'cover', borderRadius: 8 }} />;
    } else if (file.type.startsWith('video/')) {
      return <video src={url} style={{ width: 80, height: 80, objectFit: 'cover', borderRadius: 8 }} muted />;
    }
    return <div style={{ width: 80, height: 80, display: 'flex', alignItems: 'center', justifyContent: 'center', backgroundColor: 'var(--surface-100)', borderRadius: 8, fontSize: '1.5rem' }}>📄</div>;
  };

  return (
    <section className="card lab-panel" style={{ background: 'var(--bg-glass)', backdropFilter: 'blur(20px)', border: '1px solid var(--border)', borderRadius: 16, padding: 24 }}>
      <div className="lab-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <h2 style={{ fontSize: '1.5rem', fontWeight: 700, background: 'linear-gradient(135deg, #6366f1, #a855f7)', WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent' }}>
          مختبر الوسائط المطور (Media Lab Pro)
        </h2>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <span style={{ fontSize: '0.85rem', color: 'var(--muted)' }}>حالة الاتصال بالخادم:</span>
          <span className={`status-dot`} style={{
            display: 'inline-block',
            width: 12,
            height: 12,
            borderRadius: '50%',
            backgroundColor: mediaSocket.connectionState === 'CONNECTED' ? '#10b981' : '#ef4444',
            boxShadow: mediaSocket.connectionState === 'CONNECTED' ? '0 0 10px #10b981' : '0 0 10px #ef4444'
          }} />
        </div>
      </div>

      <div 
        onDragEnter={handleDrag}
        onDragOver={handleDrag}
        onDragLeave={handleDrag}
        onDrop={handleDrop}
        style={{
          border: `2px dashed ${dragActive ? 'var(--primary)' : 'var(--border)'}`,
          borderRadius: 12,
          padding: '40px 20px',
          textAlign: 'center',
          backgroundColor: dragActive ? 'var(--primary-glow)' : 'var(--bg2)',
          transition: 'all 0.3s ease',
          cursor: 'pointer',
          marginBottom: 24
        }}
        onClick={() => document.getElementById('media-file-input')?.click()}
      >
        <input 
          id="media-file-input" 
          type="file" 
          multiple 
          style={{ display: 'none' }} 
          onChange={handleFileInput}
          accept="image/*,video/*,application/pdf"
        />
        <div style={{ fontSize: '3rem', marginBottom: 12 }}>☁️</div>
        <p style={{ margin: '0 0 8px 0', fontWeight: 600 }}>اسحب وأفلت الملفات هنا أو اضغط للاختيار</p>
        <p style={{ margin: 0, fontSize: '0.8rem', color: 'var(--muted)' }}>صور • فيديو • مستندات PDF</p>
      </div>

      <div className="queue-list" style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        {queue.length === 0 ? (
          <p style={{ textAlign: 'center', color: 'var(--muted)', marginTop: 20 }}>قائمة الانتظار فارغة. ابدأ بإضافة بعض الملفات.</p>
        ) : (
          queue.map((item) => {
            const progressPercent = Math.round((item.progress.uploadedBytes / item.progress.totalBytes) * 100) || 0;
            const uploadSpeed = formatBytes(item.progress.speedBytesPerSec) + '/s';
            
            return (
              <div key={item.id} style={{
                display: 'flex',
                gap: 16,
                padding: 16,
                backgroundColor: 'var(--bg2)',
                borderRadius: 12,
                border: '1px solid var(--border)',
                position: 'relative',
                overflow: 'hidden'
              }}>
                <div className="local-preview">
                  {getPreviewElement(item.file)}
                </div>
                
                <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 6 }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <span style={{ fontWeight: 600, fontSize: '0.95rem' }}>{item.file.name}</span>
                    <span style={{ fontSize: '0.8rem', color: 'var(--muted)' }}>{formatBytes(item.file.size)}</span>
                  </div>

                  {/* Upload Info */}
                  <div style={{ display: 'flex', gap: 12, fontSize: '0.8rem', color: 'var(--muted)' }}>
                    <span>المرحلة: <strong>{item.progress.phase}</strong></span>
                    {item.progress.phase === 'UPLOADING' && (
                      <>
                        <span>السرعة: <strong>{uploadSpeed}</strong></span>
                        <span>التقدم: <strong>{progressPercent}%</strong></span>
                      </>
                    )}
                  </div>

                  {/* Progress Bar */}
                  {item.progress.phase !== 'IDLE' && item.progress.phase !== 'DONE' && (
                    <div style={{ height: 6, backgroundColor: 'var(--border)', borderRadius: 3, overflow: 'hidden', marginTop: 4 }}>
                      <div style={{
                        height: '100%',
                        width: `${progressPercent}%`,
                        backgroundColor: 'var(--primary)',
                        transition: 'width 0.2s ease-out'
                      }} />
                    </div>
                  )}

                  {/* Processing Status from WebSocket */}
                  {item.processingStatus && (
                    <div style={{ display: 'flex', gap: 8, alignItems: 'center', fontSize: '0.85rem', color: 'var(--warning)', marginTop: 4 }}>
                      <span className="pulsing-icon">⚙️</span>
                      <span>معالجة: {item.processingStatus} 
                        {item.processingProgress !== null ? ` (${item.processingProgress}%)` : ''}
                      </span>
                    </div>
                  )}

                  {/* Error Notification */}
                  {item.progress.error && (
                    <div style={{ color: 'var(--error)', fontSize: '0.8rem', marginTop: 4 }}>
                      ⚠️ {item.progress.error}
                    </div>
                  )}

                  {/* Actions buttons */}
                  <div style={{ display: 'flex', gap: 10, marginTop: 8, flexWrap: 'wrap' }}>
                    {item.progress.phase === 'IDLE' && (
                      <button className="btn btn-primary btn-sm" onClick={() => startUpload(item.id)}>بدء الرفع</button>
                    )}
                    {item.progress.phase === 'UPLOADING' && (
                      <button className="btn btn-sm" style={{ backgroundColor: 'var(--warning)' }} onClick={() => pauseUpload(item.id)}>إيقاف مؤقت</button>
                    )}
                    {item.progress.phase === 'PAUSED' && (
                      <button className="btn btn-primary btn-sm" onClick={() => resumeUpload(item.id)}>استكمال</button>
                    )}
                    {item.progress.phase !== 'IDLE' && item.progress.phase !== 'DONE' && item.progress.phase !== 'ERROR' && (
                      <button className="btn btn-sm" style={{ backgroundColor: 'var(--error)' }} onClick={() => cancelUpload(item.id)}>إلغاء</button>
                    )}
                    {item.progress.phase === 'DONE' && item.mediaId && (
                      <>
                        <button className="btn btn-primary btn-sm" disabled={item.downloading} onClick={() => downloadFile(item)}>
                          {item.downloading ? `تحميل ${item.downloadProgress}%` : 'تحميل'}
                        </button>
                        <button className="btn btn-sm" style={{ backgroundColor: 'var(--accent, #6366f1)' }} disabled={item.copyingLink} onClick={() => copyLink(item)}>
                          {item.copyingLink ? 'جاري النسخ...' : item.downloadUrl ? 'تم النسخ' : 'نسخ الرابط'}
                        </button>
                        <button className="btn btn-sm" style={{ backgroundColor: 'var(--error)' }} onClick={() => deleteItem(item)}>
                          حذف
                        </button>
                      </>
                    )}
                    {item.progress.phase === 'ERROR' && (
                      <button className="btn btn-ghost btn-sm" onClick={() => cancelUpload(item.id)}>إزالة</button>
                    )}
                  </div>

                  {/* Download URL display */}
                  {item.downloadUrl && (
                    <div style={{ marginTop: 6, fontSize: '0.75rem', color: 'var(--muted)', wordBreak: 'break-all', padding: '6px 10px', backgroundColor: 'var(--surface-200, rgba(255,255,255,0.05))', borderRadius: 6, border: '1px solid var(--border)' }}>
                      <span style={{ color: 'var(--success, #10b981)', marginRight: 4 }}>✓</span>
                      <a href={item.downloadUrl} target="_blank" rel="noopener noreferrer" style={{ color: 'var(--primary)', textDecoration: 'underline' }}>
                        {item.downloadUrl}
                      </a>
                    </div>
                  )}
                </div>
              </div>
            );
          })
        )}
      </div>
    </section>
  );
}
