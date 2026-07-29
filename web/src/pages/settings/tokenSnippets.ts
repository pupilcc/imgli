/** Build copy-paste client snippets for API Token settings. */

export type SnippetKind = 'curl' | 'picgo' | 'sharex' | 'cli'

export interface IntegrationSnippet {
  kind: SnippetKind
  /** Body to show / copy */
  text: string
}

/** Normalize public base URL: trim, drop trailing slash. */
export function normalizeSnippetBaseURL(raw: string): string {
  return raw.trim().replace(/\/+$/, '')
}

/**
 * @param baseURL public link/API origin (no trailing slash preferred)
 * @param token plain token if available (create-once); otherwise a placeholder label
 */
export function buildIntegrationSnippets(baseURL: string, token: string): IntegrationSnippet[] {
  const base = normalizeSnippetBaseURL(baseURL) || 'https://your-host'
  const tok = token.trim() || 'YOUR_TOKEN'
  const auth = `Bearer ${tok}`
  const uploadURL = `${base}/api/v1/upload`

  const curl = `curl -sS -X POST '${uploadURL}' \\
  -H "Authorization: ${auth}" \\
  -F 'file=@shot.png' \\
  -F 'visibility=public'`

  const picgo = `{
  "picBed": {
    "uploader": "web-uploader",
    "web-uploader": {
      "url": "${uploadURL}",
      "paramName": "file",
      "jsonPath": "data.links.url",
      "customHeader": "{\\"Authorization\\":\\"${auth}\\"}"
    }
  }
}`

  const sharex = `{
  "Version": "15.0.0",
  "Name": "imgli",
  "DestinationType": "ImageUploader",
  "RequestMethod": "POST",
  "RequestURL": "${uploadURL}",
  "Headers": {
    "Authorization": "${auth}"
  },
  "Body": "MultipartFormData",
  "FileFormName": "file",
  "Arguments": {
    "visibility": "public"
  },
  "URL": "{json:data.links.url}",
  "ThumbnailURL": "{json:data.links.thumbnail_url}",
  "ErrorMessage": "{json:message}"
}`

  const cli = `export IMGLI_BASE_URL='${base}'
export IMGLI_TOKEN='${tok}'
imgli upload shot.png`

  return [
    { kind: 'curl', text: curl },
    { kind: 'picgo', text: picgo },
    { kind: 'sharex', text: sharex },
    { kind: 'cli', text: cli },
  ]
}
