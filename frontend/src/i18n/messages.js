// 应用五语言文案字典。键统一按“域.子键”组织，{name} 为可替换占位符。
// 新增文案时请同时在五个语言中补充，避免回退到键名。

export const SUPPORTED_LOCALES = [
  { value: 'zh-CN', label: '简体中文' },
  { value: 'zh-TW', label: '繁體中文' },
  { value: 'en', label: 'English' },
  { value: 'ja', label: '日本語' },
  { value: 'ko', label: '한국어' }
]

export const messages = {
  'zh-CN': {
    app: {
      brand: '捕影',
      tagline: '视频下载器',
      language: '语言',
      desktopVideoDownloader: '桌面视频下载器',
      eyebrow: 'VIDEO DOWNLOADER',
      h1: '从网页中找到视频',
      introCopy: '粘贴公开网页地址，分析可用的视频资源。',
      footerScope: '支持公开访问且非 DRM 保护的视频资源',
      footerFFmpeg: 'FFmpeg 将随应用提供'
    },
    dir: {
      label: '下载目录',
      settings: '下载设置',
      defaultBadge: '默认',
      change: '更改…',
      outputFormat: '输出格式',
      saveAsToggle: '下载时另存为…',
      profileOriginal: '原始质量（无损封装）',
      profileCompat: '兼容 MP4 (H.264/AAC)',
      firstRunTitle: '首次使用：设置下载目录',
      firstRunBody: '当前使用默认目录 {dir}。点击更改以指定自定义位置。',
      choose: '选择目录',
      operationFailed: '目录操作失败：{msg}'
    },
    common: {
      downloadFailed: '下载失败：{msg}',
      cancel: '取消',
      download: '下载',
      unknown: '未知',
      retry: '重试'
    },
    analyze: {
      title: '视频资源分析',
      source: '来源：{title}',
      subtitle: '粘贴网页地址，识别其中的视频资源',
      urlPlaceholder: '粘贴网页 URL，例如 https://example.com/video',
      urlLabel: '网页地址',
      analyzing: '分析中…',
      analyze: '分析',
      emptyTitle: '输入 URL 开始分析',
      emptyHint: '支持公开可访问的 HTTP/HTTPS 网页。',
      loadingTitle: '正在分析页面…',
      loadingHint: '这通常需要几秒，取决于页面大小和网络情况。',
      canceledTitle: '分析已取消',
      canceledHint: '重新输入地址即可再次尝试。',
      failedTitle: '分析失败',
      failedHint: '请确认地址正确、网络通畅，且目标页面是公开可访问的。',
      unknownError: '未知错误',
      emptyResultTitle: '未发现可下载的视频资源',
      supportsLabel: '捕影 目前支持：',
      supportsVideoTag: '静态 HTML 中的 <video> 和 <source> 标签',
      supportsDirect: '直接的媒体文件链接（.mp4、.webm、.mkv 等）',
      supportsHls: '公开的 HLS (.m3u8) 和 DASH (.mpd) 清单',
      supportsNot: '不支持：需要 JavaScript 渲染的动态页面、登录/付费内容、DRM 保护资源。'
    },
    auth: {
      title: '此网站可能需要登录会话',
      body: '只有在你明确授权后，捕影 才会从所选浏览器读取当前会话。Cookie 不会展示、上传或写入分析结果。',
      browser: '浏览器',
      profileName: '配置名称（可选）',
      profilePlaceholder: '例如 Default',
      authorizeAndRetry: '授权并重试'
    },
    media: {
      unnamed: '未命名资源',
      direct: '直链',
      hasAudio: '有音频',
      hasVideo: '有视频',
      streamUnknown: '流信息未知',
      resolution: '分辨率',
      format: '格式',
      duration: '时长',
      size: '大小',
      variants: '清晰度 / 变体',
      defaultVariant: '默认'
    },
    task: {
      title: '下载任务',
      activeSummary: '活跃 {active} · 完成 {completed}',
      failedCount: '失败 {failed}',
      none: '暂无下载任务',
      waitTitle: '等待下载任务',
      waitHint: '分析视频资源并点击下载按钮，任务将出现在这里。',
      loadFailed: '无法加载任务',
      unnamed: '未命名任务',
      calculating: '计算中…',
      etaPrefix: '约 {eta}',
      savedTo: '已保存到',
      canceled: '任务已取消',
      failed: '任务失败',
      cancelFailTitle: '取消任务失败',
      retryFailTitle: '重试任务失败',
      openFileFailTitle: '打开文件失败',
      openFolderFailTitle: '定位文件失败',
      openFile: '打开文件',
      openFolder: '打开目录',
      state: {
        queued: '等待中',
        preparing: '准备中',
        downloading: '下载中',
        merging: '封装中',
        transcoding: '转码中',
        completed: '已完成',
        canceled: '已取消',
        failed: '失败'
      }
    },
    ytdlp: {
      title: '网站解析能力',
      notDetected: '未检测到内置解析器',
      current: '当前 {version}',
      checking: '检查中…',
      updating: '更新中…',
      checkUpdate: '检查并更新 yt-dlp',
      privacy: '默认不读取浏览器 Cookie；需要时会单独征得授权。',
      updateFailed: '更新失败，请稍后重试。',
      source: { bundled: '随应用提供', userUpdate: '用户更新', unavailable: '不可用' }
    },
    validate: {
      enterUrl: '请输入网页地址',
      invalidFormat: '地址格式不正确',
      schemeOnly: '仅支持 http 和 https 地址',
      noHost: '地址缺少主机名',
      noCredential: '地址中不应包含用户凭据'
    },
    errors: {
      analyzer: {
        invalid_url: '网址无效或无法解析。',
        address_rejected: '该地址不是公网可访问的地址，已被拒绝。',
        too_many_redirects: '页面重定向次数过多。',
        body_too_large: '页面内容过大，无法分析。'
      },
      ytdlp: {
        auth_required: '此网站需要登录或授权才能访问。',
        extractor_unsupported: '该网站暂不支持解析。',
        no_formats: '未找到可下载的视频格式。'
      },
      unknown: '发生未知错误：{msg}'
    }
  },

  'zh-TW': {
    app: {
      brand: '捕影',
      tagline: '影片下載器',
      language: '語言',
      desktopVideoDownloader: '桌面影片下載器',
      eyebrow: 'VIDEO DOWNLOADER',
      h1: '從網頁中找到影片',
      introCopy: '貼上公開網頁位址，分析可用的影片資源。',
      footerScope: '支援公開存取且非 DRM 保護的影片資源',
      footerFFmpeg: 'FFmpeg 將隨應用程式提供'
    },
    dir: {
      label: '下載目錄',
      settings: '下載設定',
      defaultBadge: '預設',
      change: '變更…',
      outputFormat: '輸出格式',
      saveAsToggle: '下載時另存為…',
      profileOriginal: '原始畫質（無損封裝）',
      profileCompat: '相容 MP4 (H.264/AAC)',
      firstRunTitle: '首次使用：設定下載目錄',
      firstRunBody: '目前使用預設目錄 {dir}。點擊變更以指定自訂位置。',
      choose: '選擇目錄',
      operationFailed: '目錄操作失敗：{msg}'
    },
    common: {
      downloadFailed: '下載失敗：{msg}',
      cancel: '取消',
      download: '下載',
      unknown: '未知',
      retry: '重試'
    },
    analyze: {
      title: '影片資源分析',
      source: '來源：{title}',
      subtitle: '貼上網頁位址，識別其中的影片資源',
      urlPlaceholder: '貼上網頁 URL，例如 https://example.com/video',
      urlLabel: '網頁位址',
      analyzing: '分析中…',
      analyze: '分析',
      emptyTitle: '輸入 URL 開始分析',
      emptyHint: '支援公開存取的 HTTP/HTTPS 網頁。',
      loadingTitle: '正在分析頁面…',
      loadingHint: '這通常需要幾秒，視頁面大小與網路狀況而定。',
      canceledTitle: '分析已取消',
      canceledHint: '重新輸入位址即可再次嘗試。',
      failedTitle: '分析失敗',
      failedHint: '請確認位址正確、網路暢通，且目標頁面是公開存取的。',
      unknownError: '未知錯誤',
      emptyResultTitle: '未發現可下載的影片資源',
      supportsLabel: '捕影 目前支援：',
      supportsVideoTag: '靜態 HTML 中的 <video> 與 <source> 標籤',
      supportsDirect: '直接的媒體檔案連結（.mp4、.webm、.mkv 等）',
      supportsHls: '公開的 HLS (.m3u8) 與 DASH (.mpd) 清單',
      supportsNot: '不支援：需要 JavaScript 渲染的動態頁面、登入/付費內容、DRM 保護資源。'
    },
    auth: {
      title: '此網站可能需要登入工作階段',
      body: '只有在你明確授權後，捕影 才會從所選瀏覽器讀取目前工作階段。Cookie 不會顯示、上傳或寫入分析結果。',
      browser: '瀏覽器',
      profileName: '設定檔名稱（選用）',
      profilePlaceholder: '例如 Default',
      authorizeAndRetry: '授權並重試'
    },
    media: {
      unnamed: '未命名資源',
      direct: '直鏈',
      hasAudio: '有音訊',
      hasVideo: '有影片',
      streamUnknown: '串流資訊未知',
      resolution: '解析度',
      format: '格式',
      duration: '時長',
      size: '大小',
      variants: '畫質 / 變體',
      defaultVariant: '預設'
    },
    task: {
      title: '下載任務',
      activeSummary: '活躍 {active} · 完成 {completed}',
      failedCount: '失敗 {failed}',
      none: '目前沒有下載任務',
      waitTitle: '等待下載任務',
      waitHint: '分析影片資源並點擊下載按鈕，任務將出現在這裡。',
      loadFailed: '無法載入任務',
      unnamed: '未命名任務',
      calculating: '計算中…',
      etaPrefix: '約 {eta}',
      savedTo: '已儲存到',
      canceled: '任務已取消',
      failed: '任務失敗',
      cancelFailTitle: '取消任務失敗',
      retryFailTitle: '重試任務失敗',
      openFileFailTitle: '開啟檔案失敗',
      openFolderFailTitle: '定位檔案失敗',
      openFile: '開啟檔案',
      openFolder: '開啟目錄',
      state: {
        queued: '等待中',
        preparing: '準備中',
        downloading: '下載中',
        merging: '封裝中',
        transcoding: '轉碼中',
        completed: '已完成',
        canceled: '已取消',
        failed: '失敗'
      }
    },
    ytdlp: {
      title: '網站解析能力',
      notDetected: '未偵測到內建解析器',
      current: '目前 {version}',
      checking: '檢查中…',
      updating: '更新中…',
      checkUpdate: '檢查並更新 yt-dlp',
      privacy: '預設不讀取瀏覽器 Cookie；需要時會另行徵得授權。',
      updateFailed: '更新失敗，請稍後再試。',
      source: { bundled: '隨應用程式提供', userUpdate: '使用者更新', unavailable: '不可用' }
    },
    validate: {
      enterUrl: '請輸入網頁位址',
      invalidFormat: '位址格式不正確',
      schemeOnly: '僅支援 http 與 https 位址',
      noHost: '位址缺少主機名稱',
      noCredential: '位址中不應包含使用者憑證'
    },
    errors: {
      analyzer: {
        invalid_url: '位址無效或無法解析。',
        address_rejected: '該位址非公網可存取的位址，已被拒絕。',
        too_many_redirects: '頁面重新導向次數過多。',
        body_too_large: '頁面內容過大，無法分析。'
      },
      ytdlp: {
        auth_required: '此網站需要登入或授權才能存取。',
        extractor_unsupported: '該網站目前不支援解析。',
        no_formats: '未找到可下載的影片格式。'
      },
      unknown: '發生未知錯誤：{msg}'
    }
  },

  en: {
    app: {
      brand: 'VidGrab',
      tagline: 'Video Downloader',
      language: 'Language',
      desktopVideoDownloader: 'Desktop Video Downloader',
      eyebrow: 'VIDEO DOWNLOADER',
      h1: 'Find videos on any web page',
      introCopy: 'Paste a public web page URL to analyze available video resources.',
      footerScope: 'Supports publicly accessible, non-DRM video resources',
      footerFFmpeg: 'FFmpeg is bundled with the app'
    },
    dir: {
      label: 'Download directory',
      settings: 'Download settings',
      defaultBadge: 'Default',
      change: 'Change…',
      outputFormat: 'Output format',
      saveAsToggle: 'Save as when downloading…',
      profileOriginal: 'Original quality (lossless)',
      profileCompat: 'Compatible MP4 (H.264/AAC)',
      firstRunTitle: 'First run: set a download directory',
      firstRunBody: 'Currently using the default directory {dir}. Click Change to pick a custom location.',
      choose: 'Choose',
      operationFailed: 'Directory operation failed: {msg}'
    },
    common: {
      downloadFailed: 'Download failed: {msg}',
      cancel: 'Cancel',
      download: 'Download',
      unknown: 'Unknown',
      retry: 'Retry'
    },
    analyze: {
      title: 'Video Resource Analysis',
      source: 'Source: {title}',
      subtitle: 'Paste a web page URL to detect video resources',
      urlPlaceholder: 'Paste a web page URL, e.g. https://example.com/video',
      urlLabel: 'Web page URL',
      analyzing: 'Analyzing…',
      analyze: 'Analyze',
      emptyTitle: 'Enter a URL to start',
      emptyHint: 'Supports publicly accessible HTTP/HTTPS pages.',
      loadingTitle: 'Analyzing page…',
      loadingHint: 'This usually takes a few seconds depending on page size and network.',
      canceledTitle: 'Analysis canceled',
      canceledHint: 'Enter a new URL to try again.',
      failedTitle: 'Analysis failed',
      failedHint: 'Make sure the URL is correct, your network is reachable, and the page is publicly accessible.',
      unknownError: 'Unknown error',
      emptyResultTitle: 'No downloadable videos found',
      supportsLabel: 'VidGrab currently supports:',
      supportsVideoTag: '<video> and <source> tags in static HTML',
      supportsDirect: 'Direct media file links (.mp4, .webm, .mkv, etc.)',
      supportsHls: 'Public HLS (.m3u8) and DASH (.mpd) manifests',
      supportsNot: 'Not supported: JS-rendered dynamic pages, login/paid content, DRM-protected resources.'
    },
    auth: {
      title: 'This site may require a login session',
      body: 'Only after your explicit authorization will VidGrab read the current session from the selected browser. Cookies are never displayed, uploaded, or written into results.',
      browser: 'Browser',
      profileName: 'Profile name (optional)',
      profilePlaceholder: 'e.g. Default',
      authorizeAndRetry: 'Authorize & retry'
    },
    media: {
      unnamed: 'Unnamed resource',
      direct: 'Direct',
      hasAudio: 'Audio',
      hasVideo: 'Video',
      streamUnknown: 'Stream info unknown',
      resolution: 'Resolution',
      format: 'Format',
      duration: 'Duration',
      size: 'Size',
      variants: 'Quality / Variants',
      defaultVariant: 'Default'
    },
    task: {
      title: 'Download Tasks',
      activeSummary: 'Active {active} · Completed {completed}',
      failedCount: 'Failed {failed}',
      none: 'No download tasks',
      waitTitle: 'Waiting for tasks',
      waitHint: 'Analyze a video and click Download; tasks will appear here.',
      loadFailed: 'Failed to load tasks',
      unnamed: 'Unnamed task',
      calculating: 'Calculating…',
      etaPrefix: 'About {eta}',
      savedTo: 'Saved to',
      canceled: 'Task canceled',
      failed: 'Task failed',
      cancelFailTitle: 'Cancel failed',
      retryFailTitle: 'Retry failed',
      openFileFailTitle: 'Open file failed',
      openFolderFailTitle: 'Reveal in folder failed',
      openFile: 'Open file',
      openFolder: 'Open folder',
      state: {
        queued: 'Queued',
        preparing: 'Preparing',
        downloading: 'Downloading',
        merging: 'Merging',
        transcoding: 'Transcoding',
        completed: 'Completed',
        canceled: 'Canceled',
        failed: 'Failed'
      }
    },
    ytdlp: {
      title: 'Site parsing',
      notDetected: 'No bundled parser detected',
      current: 'Current {version}',
      checking: 'Checking…',
      updating: 'Updating…',
      checkUpdate: 'Check & update yt-dlp',
      privacy: 'Cookies are not read by default; consent is requested separately when needed.',
      updateFailed: 'Update failed. Please try again later.',
      source: { bundled: 'Bundled', userUpdate: 'User update', unavailable: 'Unavailable' }
    },
    validate: {
      enterUrl: 'Please enter a web page URL',
      invalidFormat: 'The URL format is invalid',
      schemeOnly: 'Only http and https URLs are supported',
      noHost: 'The URL has no host name',
      noCredential: 'The URL should not contain user credentials'
    },
    errors: {
      analyzer: {
        invalid_url: 'The URL is invalid or could not be parsed.',
        address_rejected: 'The address is not publicly reachable and was rejected.',
        too_many_redirects: 'The page redirected too many times.',
        body_too_large: 'The page is too large to analyze.'
      },
      ytdlp: {
        auth_required: 'This site requires login or authorization.',
        extractor_unsupported: 'This site is not supported yet.',
        no_formats: 'No downloadable video formats were found.'
      },
      unknown: 'An unknown error occurred: {msg}'
    }
  },

  ja: {
    app: {
      brand: '捕影',
      tagline: '動画ダウンローダー',
      language: '言語',
      desktopVideoDownloader: 'デスクトップ動画ダウンローダー',
      eyebrow: 'VIDEO DOWNLOADER',
      h1: 'ウェブページから動画を見つける',
      introCopy: '公開ウェブページのURLを貼り付けて、利用可能な動画を解析します。',
      footerScope: '公開アクセス可能かつDRM保護されていない動画をサポート',
      footerFFmpeg: 'FFmpegはアプリに同梱されます'
    },
    dir: {
      label: 'ダウンロード先',
      settings: 'ダウンロード設定',
      defaultBadge: '既定',
      change: '変更…',
      outputFormat: '出力形式',
      saveAsToggle: 'ダウンロード時に名前を付けて保存…',
      profileOriginal: '元の品質（ロスレス）',
      profileCompat: '互換MP4 (H.264/AAC)',
      firstRunTitle: '初回起動：ダウンロード先を設定',
      firstRunBody: '現在は既定のフォルダ {dir} を使用しています。変更をクリックして任意の場所を指定してください。',
      choose: 'フォルダを選択',
      operationFailed: 'フォルダ操作に失敗しました: {msg}'
    },
    common: {
      downloadFailed: 'ダウンロードに失敗しました: {msg}',
      cancel: 'キャンセル',
      download: 'ダウンロード',
      unknown: '不明',
      retry: '再試行'
    },
    analyze: {
      title: '動画リソース解析',
      source: 'ソース: {title}',
      subtitle: 'ウェブページのURLを貼り付けて動画を検出',
      urlPlaceholder: 'ウェブページのURLを貼り付け（例: https://example.com/video）',
      urlLabel: 'ウェブページのURL',
      analyzing: '解析中…',
      analyze: '解析',
      emptyTitle: 'URLを入力して開始',
      emptyHint: '公開アクセス可能なHTTP/HTTPSページに対応。',
      loadingTitle: 'ページを解析中…',
      loadingHint: 'ページサイズとネットワーク状況により数秒かかることがあります。',
      canceledTitle: '解析をキャンセルしました',
      canceledHint: '新しいURLを入力して再試行できます。',
      failedTitle: '解析に失敗しました',
      failedHint: 'URLが正しいか、ネットワークに接続されているか、ページが公開アクセス可能かを確認してください。',
      unknownError: '不明なエラー',
      emptyResultTitle: 'ダウンロード可能な動画が見つかりません',
      supportsLabel: '捕影 が現在対応している形式:',
      supportsVideoTag: '静的HTML内の <video> と <source> タグ',
      supportsDirect: '直接のメディアファイルリンク（.mp4、.webm、.mkv など）',
      supportsHls: '公開HLS (.m3u8) とDASH (.mpd) マニフェスト',
      supportsNot: '非対応: JavaScriptで描画される動的ページ、ログイン/有料コンテンツ、DRM保護リソース。'
    },
    auth: {
      title: 'このサイトはログインセッションが必要な場合があります',
      body: '明示的に承認された場合のみ、捕影 は選択したブラウザから現在のセッションを読み取ります。Cookieは表示・アップロード・解析結果への書き込みは行いません。',
      browser: 'ブラウザ',
      profileName: 'プロファイル名（任意）',
      profilePlaceholder: '例: Default',
      authorizeAndRetry: '承認して再試行'
    },
    media: {
      unnamed: '名前のないリソース',
      direct: '直リンク',
      hasAudio: '音声あり',
      hasVideo: '動画あり',
      streamUnknown: 'ストリーム情報不明',
      resolution: '解像度',
      format: '形式',
      duration: '長さ',
      size: 'サイズ',
      variants: '画質 / バリアント',
      defaultVariant: '既定'
    },
    task: {
      title: 'ダウンロードタスク',
      activeSummary: '実行中 {active} · 完了 {completed}',
      failedCount: '失敗 {failed}',
      none: 'ダウンロードタスクはありません',
      waitTitle: 'タスク待機中',
      waitHint: '動画を解析してダウンロードボタンを押すと、ここにタスクが表示されます。',
      loadFailed: 'タスクを読み込めませんでした',
      unnamed: '名前のないタスク',
      calculating: '計算中…',
      etaPrefix: '約 {eta}',
      savedTo: '保存先',
      canceled: 'タスクはキャンセルされました',
      failed: 'タスクは失敗しました',
      cancelFailTitle: 'キャンセルに失敗しました',
      retryFailTitle: '再試行に失敗しました',
      openFileFailTitle: 'ファイルを開けませんでした',
      openFolderFailTitle: 'フォルダを表示できませんでした',
      openFile: 'ファイルを開く',
      openFolder: 'フォルダを開く',
      state: {
        queued: '待機中',
        preparing: '準備中',
        downloading: 'ダウンロード中',
        merging: '結合中',
        transcoding: '変換中',
        completed: '完了',
        canceled: 'キャンセル',
        failed: '失敗'
      }
    },
    ytdlp: {
      title: 'サイト解析能力',
      notDetected: '内蔵パーサーを検出できません',
      current: '現在 {version}',
      checking: '確認中…',
      updating: '更新中…',
      checkUpdate: 'yt-dlpを確認して更新',
      privacy: '既定ではブラウザのCookieを読み取りません。必要な場合のみ別途承認を求めます。',
      updateFailed: '更新に失敗しました。後でもう一度お試しください。',
      source: { bundled: '同梱', userUpdate: 'ユーザー更新', unavailable: '利用不可' }
    },
    validate: {
      enterUrl: 'ウェブページのURLを入力してください',
      invalidFormat: 'URLの形式が正しくありません',
      schemeOnly: 'http と https のURLのみ対応',
      noHost: 'URLにホスト名がありません',
      noCredential: 'URLにユーザー資格情報を含めないでください'
    },
    errors: {
      analyzer: {
        invalid_url: 'URLが無効か、解析できませんでした。',
        address_rejected: 'このアドレスは公開ネットワークからアクセスできないため拒否されました。',
        too_many_redirects: 'ページのリダイレクト回数が多すぎます。',
        body_too_large: 'ページが大きすぎて解析できません。'
      },
      ytdlp: {
        auth_required: 'このサイトへのアクセスにはログインまたは承認が必要です。',
        extractor_unsupported: 'このサイトはまだ対応していません。',
        no_formats: 'ダウンロード可能な動画形式が見つかりませんでした。'
      },
      unknown: '不明なエラーが発生しました: {msg}'
    }
  },

  ko: {
    app: {
      brand: '捕影',
      tagline: '동영상 다운로더',
      language: '언어',
      desktopVideoDownloader: '데스크톱 동영상 다운로더',
      eyebrow: 'VIDEO DOWNLOADER',
      h1: '웹페이지에서 동영상 찾기',
      introCopy: '공개 웹페이지 주소를 붙여넣어 사용 가능한 동영상을 분석합니다.',
      footerScope: '공개 접근 가능하고 DRM으로 보호되지 않은 동영상 지원',
      footerFFmpeg: 'FFmpeg는 앱에 포함되어 제공됩니다'
    },
    dir: {
      label: '다운로드 폴더',
      settings: '다운로드 설정',
      defaultBadge: '기본',
      change: '변경…',
      outputFormat: '출력 형식',
      saveAsToggle: '다운로드 시 다른 이름으로 저장…',
      profileOriginal: '원본 품질(무손실)',
      profileCompat: '호환 MP4 (H.264/AAC)',
      firstRunTitle: '첫 실행: 다운로드 폴더 설정',
      firstRunBody: '현재 기본 폴더 {dir} 을(를) 사용 중입니다. 변경을 눌러 원하는 위치를 지정하세요.',
      choose: '폴더 선택',
      operationFailed: '폴더 작업에 실패했습니다: {msg}'
    },
    common: {
      downloadFailed: '다운로드에 실패했습니다: {msg}',
      cancel: '취소',
      download: '다운로드',
      unknown: '알 수 없음',
      retry: '다시 시도'
    },
    analyze: {
      title: '동영상 리소스 분석',
      source: '출처: {title}',
      subtitle: '웹페이지 주소를 붙여넣어 동영상을 감지합니다',
      urlPlaceholder: '웹페이지 URL을 붙여넣으세요 (예: https://example.com/video)',
      urlLabel: '웹페이지 URL',
      analyzing: '분석 중…',
      analyze: '분석',
      emptyTitle: 'URL을 입력해 시작',
      emptyHint: '공개 접근 가능한 HTTP/HTTPS 페이지를 지원합니다.',
      loadingTitle: '페이지 분석 중…',
      loadingHint: '페이지 크기와 네트워크 상태에 따라 몇 초 걸릴 수 있습니다.',
      canceledTitle: '분석이 취소되었습니다',
      canceledHint: '새 주소를 입력하면 다시 시도할 수 있습니다.',
      failedTitle: '분석에 실패했습니다',
      failedHint: '주소가 올바른지, 네트워크가 연결되었는지, 페이지가 공개 접근 가능한지 확인하세요.',
      unknownError: '알 수 없는 오류',
      emptyResultTitle: '다운로드 가능한 동영상을 찾지 못했습니다',
      supportsLabel: '捕影 현재 지원:',
      supportsVideoTag: '정적 HTML의 <video> 및 <source> 태그',
      supportsDirect: '직접 미디어 파일 링크(.mp4, .webm, .mkv 등)',
      supportsHls: '공개 HLS(.m3u8) 및 DASH(.mpd) 매니페스트',
      supportsNot: '미지원: JavaScript 렌더링 동적 페이지, 로그인/유료 콘텐츠, DRM 보호 리소스.'
    },
    auth: {
      title: '이 사이트는 로그인 세션이 필요할 수 있습니다',
      body: '명시적으로 승인한 경우에만 捕影 이(가) 선택한 브라우저에서 현재 세션을 읽습니다. 쿠키는 표시, 업로드 또는 결과에 기록되지 않습니다.',
      browser: '브라우저',
      profileName: '프로필 이름(선택)',
      profilePlaceholder: '예: Default',
      authorizeAndRetry: '승인하고 다시 시도'
    },
    media: {
      unnamed: '이름 없는 리소스',
      direct: '직접 링크',
      hasAudio: '오디오 있음',
      hasVideo: '동영상 있음',
      streamUnknown: '스트림 정보 알 수 없음',
      resolution: '해상도',
      format: '형식',
      duration: '길이',
      size: '크기',
      variants: '화질 / 변형',
      defaultVariant: '기본'
    },
    task: {
      title: '다운로드 작업',
      activeSummary: '활성 {active} · 완료 {completed}',
      failedCount: '실패 {failed}',
      none: '다운로드 작업 없음',
      waitTitle: '작업 대기 중',
      waitHint: '동영상을 분석하고 다운로드 버튼을 누르면 작업이 여기에 표시됩니다.',
      loadFailed: '작업을 불러오지 못했습니다',
      unnamed: '이름 없는 작업',
      calculating: '계산 중…',
      etaPrefix: '약 {eta}',
      savedTo: '저장 위치',
      canceled: '작업이 취소되었습니다',
      failed: '작업에 실패했습니다',
      cancelFailTitle: '취소에 실패했습니다',
      retryFailTitle: '다시 시도에 실패했습니다',
      openFileFailTitle: '파일 열기에 실패했습니다',
      openFolderFailTitle: '폴더 찾기에 실패했습니다',
      openFile: '파일 열기',
      openFolder: '폴더 열기',
      state: {
        queued: '대기 중',
        preparing: '준비 중',
        downloading: '다운로드 중',
        merging: '병합 중',
        transcoding: '변환 중',
        completed: '완료',
        canceled: '취소됨',
        failed: '실패'
      }
    },
    ytdlp: {
      title: '사이트 파싱 기능',
      notDetected: '내장 파서를 감지하지 못했습니다',
      current: '현재 {version}',
      checking: '확인 중…',
      updating: '업데이트 중…',
      checkUpdate: 'yt-dlp 확인 및 업데이트',
      privacy: '기본적으로 브라우저 쿠키를 읽지 않습니다. 필요 시 별도 승인을 요청합니다.',
      updateFailed: '업데이트에 실패했습니다. 나중에 다시 시도하세요.',
      source: { bundled: '앱 내장', userUpdate: '사용자 업데이트', unavailable: '사용 불가' }
    },
    validate: {
      enterUrl: '웹페이지 URL을 입력하세요',
      invalidFormat: 'URL 형식이 올바르지 않습니다',
      schemeOnly: 'http 및 https URL만 지원합니다',
      noHost: 'URL에 호스트 이름이 없습니다',
      noCredential: 'URL에 사용자 인증 정보를 포함하면 안 됩니다'
    },
    errors: {
      analyzer: {
        invalid_url: 'URL이 유효하지 않거나 파싱할 수 없습니다.',
        address_rejected: '이 주소는 공개 네트워크에서 접근할 수 없어 거부되었습니다.',
        too_many_redirects: '페이지 리디렉션 횟수가 너무 많습니다.',
        body_too_large: '페이지가 너무 커서 분석할 수 없습니다.'
      },
      ytdlp: {
        auth_required: '이 사이트에 접근하려면 로그인 또는 승인이 필요합니다.',
        extractor_unsupported: '이 사이트는 아직 지원되지 않습니다.',
        no_formats: '다운로드 가능한 동영상 형식을 찾지 못했습니다.'
      },
      unknown: '알 수 없는 오류가 발생했습니다: {msg}'
    }
  }
}
