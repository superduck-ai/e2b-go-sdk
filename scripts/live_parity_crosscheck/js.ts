import { mkdtempSync, mkdirSync, rmSync, symlinkSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { pathToFileURL } from 'node:url'

type Result = {
  language: 'js'
  case:
    | 'claude'
    | 'claude_derived'
    | 'randomness'
    | 'randomness_alias'
    | 'volume'
    | 'volume_api_payload'
    | 'ubuntu'
    | 'template_timeout'
    | 'template_methods'
    | 'config_headers'
    | 'metrics'
    | 'network_rules'
    | 'network_egress'
    | 'network_update_payload'
    | 'template_api_payload'
    | 'debug_root'
  status: string
  detail?: string
  extra?: Record<string, string>
}

const numpyRandomCommand = `python3 - <<'PY'
import numpy as np
print([np.random.normal(), np.random.normal(), np.random.normal()])
PY`

const claudeDerivedNumpyInstallCommand =
  'python3 -m pip install --break-system-packages --no-cache-dir numpy'
const randomnessAliasTemplate = 'en716jw99aj63v1k8ugh'
const upstreamRandomnessAliasCommand =
  'python -c "import numpy as np; print([np.random.normal(),np.random.normal(),np.random.normal()])"'

const templateMethodsSummaryCommand = `printf 'runtime_user=%s\n' "$(whoami)"
printf 'bashrc_target=%s\n' "$(readlink /home/user/.bashrc.local)"
printf 'preserved_type=%s\n' "$(if [ -L /app/link-preserved.txt ]; then echo symlink; else echo regular; fi)"
printf 'preserved_target=%s\n' "$(readlink /app/link-preserved.txt)"
printf 'preserved_content=%s\n' "$(cat /app/link-preserved.txt)"
printf 'resolved_type=%s\n' "$(if [ -L /app/link-resolved.txt ]; then echo symlink; else echo regular; fi)"
printf 'resolved_content=%s\n' "$(cat /app/link-resolved.txt)"`

const expectedTemplateMethodsSummary = [
  'runtime_user=user',
  'bashrc_target=.bashrc',
  'preserved_type=symlink',
  'preserved_target=test.txt',
  'preserved_content=template symlink content',
  'resolved_type=regular',
  'resolved_content=template symlink content',
].join('\n')

const JS_SDK_DIR = process.env.E2B_JS_SDK_DIR ?? '/data/E2B/packages/js-sdk'
const sdk = await import(pathToFileURL(`${JS_SDK_DIR}/src/index.ts`).href)
const { ALL_TRAFFIC, Sandbox, Volume, Template, waitForTimeout } = sdk

const caseName = parseCase()
const results: Result[] = []

for (const current of selectedCases(caseName)) {
  switch (current) {
    case 'claude':
      results.push(await runClaudeCase())
      break
    case 'claude_derived':
      results.push(await runClaudeDerivedCase())
      break
    case 'randomness':
      results.push(await runRandomnessCase())
      break
    case 'randomness_alias':
      results.push(await runRandomnessAliasCase())
      break
    case 'volume':
      results.push(await runVolumeCase())
      break
    case 'volume_api_payload':
      results.push(await runVolumeApiPayloadCase())
      break
    case 'ubuntu':
      results.push(await runUbuntuCase())
      break
    case 'template_timeout':
      results.push(await runTemplateTimeoutCase())
      break
    case 'template_methods':
      results.push(await runTemplateMethodsCase())
      break
    case 'config_headers':
      results.push(await runConfigHeadersCase())
      break
    case 'metrics':
      results.push(await runMetricsCase())
      break
    case 'network_rules':
      results.push(await runNetworkRulesCase())
      break
    case 'network_egress':
      results.push(await runNetworkEgressCase())
      break
    case 'network_update_payload':
      results.push(await runNetworkUpdatePayloadCase())
      break
    case 'template_api_payload':
      results.push(await runTemplateApiPayloadCase())
      break
    case 'debug_root':
      results.push(await runDebugRootCase())
      break
    default:
      throw new Error(`unsupported case ${current}`)
  }
}

console.log(JSON.stringify(results, null, 2))

function parseCase() {
  const index = process.argv.indexOf('--case')
  return index >= 0 ? process.argv[index + 1] ?? 'all' : 'all'
}

function selectedCases(caseName: string) {
  if (caseName === 'all') {
    return [
      'claude',
      'claude_derived',
      'randomness',
      'randomness_alias',
      'volume',
      'volume_api_payload',
      'ubuntu',
      'template_timeout',
      'template_methods',
      'config_headers',
      'metrics',
      'network_rules',
      'network_egress',
      'network_update_payload',
      'template_api_payload',
      'debug_root',
    ] as const
  }
  if (
    caseName === 'claude' ||
    caseName === 'claude_derived' ||
    caseName === 'randomness' ||
    caseName === 'randomness_alias' ||
    caseName === 'volume' ||
    caseName === 'volume_api_payload' ||
    caseName === 'ubuntu' ||
    caseName === 'template_timeout' ||
    caseName === 'template_methods' ||
    caseName === 'config_headers' ||
    caseName === 'metrics' ||
    caseName === 'network_rules' ||
    caseName === 'network_egress' ||
    caseName === 'network_update_payload' ||
    caseName === 'template_api_payload' ||
    caseName === 'debug_root'
  ) {
    return [caseName] as const
  }
  throw new Error(`unsupported case ${caseName}`)
}

async function runDebugRootCase(): Promise<Result> {
  const previousDebug = process.env.E2B_DEBUG
  const previousApiUrl = process.env.E2B_API_URL
  const previousDomain = process.env.E2B_DOMAIN

  process.env.E2B_DEBUG = 'true'
  delete process.env.E2B_API_URL
  delete process.env.E2B_DOMAIN

  try {
    const { ConnectionConfig } = await import(
      pathToFileURL(`${JS_SDK_DIR}/src/connectionConfig.ts`).href
    )
    const config = new ConnectionConfig({ debug: false })

    const status =
      config.debug === true && config.apiUrl === 'http://localhost:3000'
        ? 'ok'
        : 'mismatch'
    const detail =
      status === 'ok'
        ? 'env debug=true wins over explicit debug=false at root connection-config construction'
        : `unexpected root debug semantics: debug=${String(config.debug)} apiUrl=${String(config.apiUrl)}`

    return {
      language: 'js',
      case: 'debug_root',
      status,
      detail,
      extra: {
        env_debug: 'true',
        arg_debug: 'false',
        debug: String(config.debug),
        api_url: String(config.apiUrl),
      },
    }
  } finally {
    restoreEnv('E2B_DEBUG', previousDebug)
    restoreEnv('E2B_API_URL', previousApiUrl)
    restoreEnv('E2B_DOMAIN', previousDomain)
  }
}

function connectionOpts() {
  return {
    apiKey: process.env.E2B_API_KEY,
    accessToken: process.env.E2B_ACCESS_TOKEN,
    domain: process.env.E2B_DOMAIN,
    apiUrl: process.env.E2B_API_URL,
    requestTimeoutMs: 120_000,
  }
}

function buildOpts() {
  return {
    ...connectionOpts(),
    cpuCount: 1,
    memoryMB: 512,
    requestTimeoutMs: 600_000,
  }
}

function defaultTimeoutBuildOpts() {
  return {
    apiKey: process.env.E2B_API_KEY,
    accessToken: process.env.E2B_ACCESS_TOKEN,
    domain: process.env.E2B_DOMAIN,
    apiUrl: process.env.E2B_API_URL,
    cpuCount: 1,
    memoryMB: 1024,
    skipCache: true,
  }
}

async function runClaudeCase(): Promise<Result> {
  const template = 'claude-code-interpreter'

  try {
    const exists = await Template.exists(template, connectionOpts())
    if (!exists) {
      return {
        language: 'js',
        case: 'claude',
        status: 'template_missing',
        detail: 'claude-code-interpreter template alias is unavailable',
      }
    }
  } catch (error) {
    return toErrorResult('claude', error)
  }

  let sandbox: any | undefined
  try {
    sandbox = await Sandbox.create(template, {
      ...connectionOpts(),
      timeoutMs: 10 * 60_000,
    })

    const result = await sandbox.commands.run(numpyRandomCommand)

    return {
      language: 'js',
      case: 'claude',
      status: 'ok',
      detail: String(result.stdout).trim(),
    }
  } catch (error) {
    return classifyCommandError('claude', error)
  } finally {
    if (sandbox) {
      try {
        await sandbox.kill()
      } catch {}
    }
  }
}

async function runRandomnessCase(): Promise<Result> {
  return runBuiltNumpyTemplateCase(
    'randomness',
    Template()
      .fromPythonImage('3.12')
      .skipCache()
      .runCmd('python3 -m pip install --no-cache-dir numpy'),
    `js-sdk-randomness-crosscheck-${Date.now()}`,
  )
}

async function runRandomnessAliasCase(): Promise<Result> {
  try {
    const exists = await Template.exists(randomnessAliasTemplate, connectionOpts())
    if (!exists) {
      return {
        language: 'js',
        case: 'randomness_alias',
        status: 'template_missing',
        detail: 'upstream randomness alias is unavailable',
      }
    }
  } catch (error) {
    return toErrorResult('randomness_alias', error)
  }

  let firstSandbox: any | undefined
  let secondSandbox: any | undefined
  try {
    firstSandbox = await Sandbox.create(randomnessAliasTemplate, {
      ...connectionOpts(),
      timeoutMs: 10 * 60_000,
    })
    const first = await firstSandbox.commands.run(upstreamRandomnessAliasCommand)

    let second: any
    try {
      second = await firstSandbox.commands.run(upstreamRandomnessAliasCommand)
    } catch (error) {
      return {
        language: 'js',
        case: 'randomness_alias',
        status: 'partial',
        detail: errorDetail(error),
        extra: {
          phase: 'same_sandbox_second_command',
          template_id: randomnessAliasTemplate,
        },
      }
    }

    if (String(first.stdout).trim() === String(second.stdout).trim()) {
      return {
        language: 'js',
        case: 'randomness_alias',
        status: 'error',
        detail: 'expected different random vectors in the same sandbox',
        extra: {
          phase: 'same_sandbox_compare',
          template_id: randomnessAliasTemplate,
        },
      }
    }

    secondSandbox = await Sandbox.create(randomnessAliasTemplate, {
      ...connectionOpts(),
      timeoutMs: 10 * 60_000,
    })
    const third = await secondSandbox.commands.run(upstreamRandomnessAliasCommand)
    if (String(first.stdout).trim() === String(third.stdout).trim()) {
      return {
        language: 'js',
        case: 'randomness_alias',
        status: 'error',
        detail: 'expected different random vectors across sandboxes from the same alias',
        extra: {
          phase: 'cross_sandbox_compare',
          template_id: randomnessAliasTemplate,
        },
      }
    }

    return {
      language: 'js',
      case: 'randomness_alias',
      status: 'ok',
      detail: 'same-sandbox and cross-sandbox alias randomness matched upstream expectations',
      extra: {
        template_id: randomnessAliasTemplate,
      },
    }
  } catch (error) {
    return toErrorResult('randomness_alias', error)
  } finally {
    if (firstSandbox) {
      try {
        await firstSandbox.kill()
      } catch {}
    }
    if (secondSandbox) {
      try {
        await secondSandbox.kill()
      } catch {}
    }
  }
}

async function runClaudeDerivedCase(): Promise<Result> {
  try {
    const exists = await Template.exists('claude-code-interpreter', connectionOpts())
    if (!exists) {
      return {
        language: 'js',
        case: 'claude_derived',
        status: 'template_missing',
        detail: 'claude-code-interpreter template alias is unavailable',
      }
    }
  } catch (error) {
    return toErrorResult('claude_derived', error)
  }

  return runBuiltNumpyTemplateCase(
    'claude_derived',
    Template()
      .fromTemplate('claude-code-interpreter')
      .skipCache()
      .runCmd(claudeDerivedNumpyInstallCommand),
    `js-sdk-claude-derived-crosscheck-${Date.now()}`,
    { base_template: 'claude-code-interpreter' },
  )
}

async function runBuiltNumpyTemplateCase(
  caseName: 'randomness' | 'claude_derived',
  template: any,
  name: string,
  extra?: Record<string, string>
): Promise<Result> {
  let templateId: string | undefined
  let firstSandbox: any | undefined
  let secondSandbox: any | undefined

  try {
    const buildInfo = await Template.build(template, name, buildOpts())
    templateId = buildInfo.templateId

    firstSandbox = await Sandbox.create(buildInfo.templateId, {
      ...connectionOpts(),
      timeoutMs: 10 * 60_000,
    })
    const first = await runNumpyVector(firstSandbox)
    const second = await runNumpyVector(firstSandbox)
    if (first === second) {
      return {
        language: 'js',
        case: caseName,
        status: 'error',
        detail: 'expected different random vectors in the same sandbox',
        extra: mergeExtra(extra, {
          template_id: buildInfo.templateId,
          same_sandbox_diff: 'false',
        }),
      }
    }

    secondSandbox = await Sandbox.create(buildInfo.templateId, {
      ...connectionOpts(),
      timeoutMs: 10 * 60_000,
    })
    const third = await runNumpyVector(secondSandbox)
    if (first === third) {
      return {
        language: 'js',
        case: caseName,
        status: 'error',
        detail: 'expected different random vectors across sandboxes from the same template',
        extra: mergeExtra(extra, {
          template_id: buildInfo.templateId,
          same_sandbox_diff: 'true',
          cross_sandbox_diff: 'false',
        }),
      }
    }

    return {
      language: 'js',
      case: caseName,
      status: 'ok',
      detail: 'same-sandbox and cross-sandbox numpy vectors differed',
      extra: mergeExtra(extra, {
        template_id: buildInfo.templateId,
        same_sandbox_diff: 'true',
        cross_sandbox_diff: 'true',
      }),
    }
  } catch (error) {
    const detail = errorDetail(error)
    const message = detail.toLowerCase()
    if (message.includes('404 page not found')) {
      return {
        language: 'js',
        case: caseName,
        status: 'template_api_unavailable',
        detail,
      }
    }
    if (
      message.includes('no module named') ||
      message.includes('numpy') ||
      message.includes('python3: not found')
    ) {
      return classifyCommandError(caseName, error)
    }
    return toErrorResult(caseName, error)
  } finally {
    if (firstSandbox) {
      try {
        await firstSandbox.kill()
      } catch {}
    }
    if (secondSandbox) {
      try {
        await secondSandbox.kill()
      } catch {}
    }
    if (templateId) {
      try {
        await Sandbox.deleteSnapshot(templateId, connectionOpts())
      } catch {}
    }
  }
}

async function runVolumeCase(): Promise<Result> {
  const volumeName = `js-sdk-crosscheck-${Date.now()}`
  let volume: any | undefined

  try {
    volume = await Volume.create(volumeName, connectionOpts())
    await volume.makeDir('/multi-file-dir')
    return {
      language: 'js',
      case: 'volume',
      status: 'ok',
      detail: 'makeDir(/multi-file-dir) succeeded',
    }
  } catch (error) {
    return classifyVolumeError(error)
  } finally {
    if (volume?.volumeId) {
      try {
        await Volume.destroy(volume.volumeId, connectionOpts())
      } catch {}
    }
  }
}

async function runVolumeApiPayloadCase(): Promise<Result> {
  const requests: Array<{
    method: string
    path: string
    query: Record<string, string>
    contentType: string
    authorization: string
    body: unknown
  }> = []
  const timestamp = '2026-05-30T00:00:00Z'
  const dirEntry = {
    name: 'dir',
    path: '/dir',
    type: 'directory',
    uid: 1000,
    gid: 1000,
    mode: 0o755,
    size: 0,
    atime: timestamp,
    mtime: timestamp,
    ctime: timestamp,
  }
  const fileEntry = {
    name: 'file.txt',
    path: '/file.txt',
    type: 'file',
    uid: 1000,
    gid: 1000,
    mode: 0o644,
    size: 5,
    atime: timestamp,
    mtime: timestamp,
    ctime: timestamp,
  }
  const updatedEntry = {
    name: 'dir',
    path: '/dir',
    type: 'directory',
    uid: 1001,
    gid: 1002,
    mode: 0o644,
    size: 0,
    atime: timestamp,
    mtime: timestamp,
    ctime: timestamp,
  }

  const server = Bun.serve({
    port: 0,
    async fetch(request) {
      const url = new URL(request.url)
      const text = await request.text()
      const contentType = request.headers.get('content-type') ?? ''
      requests.push({
        method: request.method,
        path: url.pathname,
        query: Object.fromEntries(url.searchParams.entries()),
        contentType,
        authorization: request.headers.get('authorization') ?? '',
        body:
          text.length === 0
            ? ''
            : contentType.startsWith('application/json')
              ? (JSON.parse(text) as Record<string, unknown>)
              : text,
      })

      if (request.method === 'GET' && url.pathname === '/volumecontent/vol-1/dir') {
        return Response.json([dirEntry])
      }
      if (request.method === 'POST' && url.pathname === '/volumecontent/vol-1/dir') {
        return Response.json(dirEntry, { status: 201 })
      }
      if (request.method === 'GET' && url.pathname === '/volumecontent/vol-1/path') {
        return Response.json(dirEntry)
      }
      if (request.method === 'PATCH' && url.pathname === '/volumecontent/vol-1/path') {
        return Response.json(updatedEntry)
      }
      if (request.method === 'GET' && url.pathname === '/volumecontent/vol-1/file') {
        return new Response('hello')
      }
      if (request.method === 'PUT' && url.pathname === '/volumecontent/vol-1/file') {
        return Response.json(fileEntry, { status: 201 })
      }
      if (request.method === 'DELETE' && url.pathname === '/volumecontent/vol-1/path') {
        return new Response(null, { status: 204 })
      }

      return new Response('unexpected path', { status: 404 })
    },
  })

  const volume = new Volume('vol-1', 'vol', 'token-1')
  const opts = {
    apiUrl: `http://127.0.0.1:${server.port}`,
    requestTimeoutMs: 1000,
  }

  try {
    const entries = await volume.list('/dir', { ...opts, depth: 2 })
    if (
      entries.length !== 1 ||
      entries[0].path !== '/dir' ||
      entries[0].type !== 'directory'
    ) {
      return {
        language: 'js',
        case: 'volume_api_payload',
        status: 'error',
        detail: `unexpected list result ${stableStringify(entries)}`,
      }
    }

    const dir = await volume.makeDir('/dir', {
      ...opts,
      uid: 1000,
      gid: 1000,
      mode: 0o755,
      force: false,
    })
    if (dir.path !== '/dir' || dir.type !== 'directory') {
      return {
        language: 'js',
        case: 'volume_api_payload',
        status: 'error',
        detail: `unexpected makeDir result ${stableStringify(dir)}`,
      }
    }

    const info = await volume.getInfo('/dir', opts)
    if (info.path !== '/dir' || info.type !== 'directory') {
      return {
        language: 'js',
        case: 'volume_api_payload',
        status: 'error',
        detail: `unexpected getInfo result ${stableStringify(info)}`,
      }
    }

    const updated = await volume.updateMetadata(
      '/dir',
      {
        uid: 1001,
        gid: 1002,
        mode: 0o644,
      },
      opts
    )
    if (updated.uid !== 1001 || updated.gid !== 1002 || updated.mode !== 0o644) {
      return {
        language: 'js',
        case: 'volume_api_payload',
        status: 'error',
        detail: `unexpected updateMetadata result ${stableStringify(updated)}`,
      }
    }

    const readValue = await volume.readFile('/file.txt', opts)
    if (readValue !== 'hello') {
      return {
        language: 'js',
        case: 'volume_api_payload',
        status: 'error',
        detail: `unexpected readFile result ${String(readValue)}`,
      }
    }

    const written = await volume.writeFile('/file.txt', 'hello', {
      ...opts,
      uid: 1000,
      gid: 1000,
      mode: 0o644,
      force: false,
    })
    if (written.path !== '/file.txt' || written.type !== 'file') {
      return {
        language: 'js',
        case: 'volume_api_payload',
        status: 'error',
        detail: `unexpected writeFile result ${stableStringify(written)}`,
      }
    }

    await volume.remove('/file.txt', opts)

    if (requests.length !== 7) {
      return {
        language: 'js',
        case: 'volume_api_payload',
        status: 'error',
        detail: `expected 7 captured requests, got ${requests.length}`,
      }
    }

    const expected = {
      list: {
        method: 'GET',
        path: '/volumecontent/vol-1/dir',
        query: {
          depth: '2',
          path: '/dir',
        },
        contentType: '',
        authorization: 'Bearer token-1',
        body: '',
      },
      make_dir: {
        method: 'POST',
        path: '/volumecontent/vol-1/dir',
        query: {
          force: 'false',
          gid: '1000',
          mode: '493',
          path: '/dir',
          uid: '1000',
        },
        contentType: '',
        authorization: 'Bearer token-1',
        body: '',
      },
      get_info: {
        method: 'GET',
        path: '/volumecontent/vol-1/path',
        query: {
          path: '/dir',
        },
        contentType: '',
        authorization: 'Bearer token-1',
        body: '',
      },
      update_metadata: {
        method: 'PATCH',
        path: '/volumecontent/vol-1/path',
        query: {
          path: '/dir',
        },
        contentType: 'application/json',
        authorization: 'Bearer token-1',
        body: {
          gid: 1002,
          mode: 420,
          uid: 1001,
        },
      },
      read_file: {
        method: 'GET',
        path: '/volumecontent/vol-1/file',
        query: {
          path: '/file.txt',
        },
        contentType: '',
        authorization: 'Bearer token-1',
        body: '',
      },
      write_file: {
        method: 'PUT',
        path: '/volumecontent/vol-1/file',
        query: {
          force: 'false',
          gid: '1000',
          mode: '420',
          path: '/file.txt',
          uid: '1000',
        },
        contentType: 'application/octet-stream',
        authorization: 'Bearer token-1',
        body: 'hello',
      },
      remove: {
        method: 'DELETE',
        path: '/volumecontent/vol-1/path',
        query: {
          path: '/file.txt',
        },
        contentType: '',
        authorization: 'Bearer token-1',
        body: '',
      },
    }

    const keys = [
      'list',
      'make_dir',
      'get_info',
      'update_metadata',
      'read_file',
      'write_file',
      'remove',
    ] as const
    const extra: Record<string, string> = {}
    for (const [index, key] of keys.entries()) {
      const actual = requests[index]
      const want = expected[key]

      extra[`${key}_method`] = actual.method
      extra[`${key}_path`] = actual.path
      extra[`${key}_query`] = stableStringify(actual.query)
      extra[`${key}_content_type`] = actual.contentType
      extra[`${key}_authorization`] = actual.authorization
      extra[`${key}_body`] = stableStringify(actual.body)

      if (actual.method !== want.method || actual.path !== want.path) {
        return {
          language: 'js',
          case: 'volume_api_payload',
          status: 'mismatch',
          detail: `${key} request target mismatch`,
          extra,
        }
      }
      if (stableStringify(actual.query) !== stableStringify(want.query)) {
        return {
          language: 'js',
          case: 'volume_api_payload',
          status: 'mismatch',
          detail: `${key} query mismatch`,
          extra,
        }
      }
      if (want.contentType) {
        if (!actual.contentType.startsWith(want.contentType)) {
          return {
            language: 'js',
            case: 'volume_api_payload',
            status: 'mismatch',
            detail: `${key} content-type mismatch`,
            extra,
          }
        }
      } else if (actual.contentType !== '') {
        return {
          language: 'js',
          case: 'volume_api_payload',
          status: 'mismatch',
          detail: `${key} content-type mismatch`,
          extra,
        }
      }
      if (actual.authorization !== want.authorization) {
        return {
          language: 'js',
          case: 'volume_api_payload',
          status: 'mismatch',
          detail: `${key} authorization mismatch`,
          extra,
        }
      }
      if (stableStringify(actual.body) !== stableStringify(want.body)) {
        return {
          language: 'js',
          case: 'volume_api_payload',
          status: 'mismatch',
          detail: `${key} payload mismatch`,
          extra,
        }
      }
    }

    return {
      language: 'js',
      case: 'volume_api_payload',
      status: 'ok',
      detail: 'captured volume content request shapes locally',
      extra,
    }
  } catch (error) {
    return toErrorResult('volume_api_payload', error)
  } finally {
    server.stop(true)
  }
}

async function runUbuntuCase(): Promise<Result> {
  const name = `js-sdk-ubuntu-crosscheck-${Date.now()}`
  try {
    const template = Template().fromImage('ubuntu:22.04').skipCache()
    const buildInfo = await Template.buildInBackground(template, name, buildOpts())

    try {
      const final = await waitForFinalBuildStatus(buildInfo.templateId, buildInfo.buildId)
      return {
        language: 'js',
        case: 'ubuntu',
        status:
          String(final.status) === 'error' &&
          final.reason?.message?.toLowerCase().includes('error waiting for provisioning sandbox')
            ? 'env_blocked'
            : String(final.status),
        detail: final.reason?.message,
        extra: {
          template_id: buildInfo.templateId,
          build_id: buildInfo.buildId,
          ...(final.reason?.step ? { reason_step: final.reason.step } : {}),
        },
      }
    } finally {
      if (buildInfo.templateId) {
        try {
          await Sandbox.deleteSnapshot(buildInfo.templateId, connectionOpts())
        } catch {}
      }
    }
  } catch (error) {
    const message = String(error)
    if (message.toLowerCase().includes('404 page not found')) {
      return {
        language: 'js',
        case: 'ubuntu',
        status: 'template_api_unavailable',
        detail: message,
      }
    }
    return toErrorResult('ubuntu', error)
  }
}

async function runTemplateTimeoutCase(): Promise<Result> {
  const tmpDir = mkdtempSync(join(tmpdir(), 'e2b-js-template-timeout-'))
  const folder = join(tmpDir, 'folder')
  mkdirSync(folder, { recursive: true })
  writeFileSync(join(folder, 'test.txt'), 'This is a test file.')

  const name = `js-sdk-template-timeout-crosscheck-${Date.now()}`
  const extra: Record<string, string> = {
    request_timeout_mode: 'default',
    status_poll_query: 'logsOffset',
    template_shape: 'fromBaseImage+copy+runCmd+setStartCmd',
    file_context_created: 'true',
    build_options_memory_mb: '1024',
    build_options_cpu: '1',
    build_options_skipcache: 'true',
  }

  let buildInfo:
    | {
        templateId: string
        buildId: string
      }
    | undefined

  const startedAt = Date.now()
  try {
    const template = Template({ fileContextPath: tmpDir })
      .fromBaseImage()
      .copy('folder/*', 'folder', { forceUpload: true })
      .runCmd('cat folder/test.txt')
      .setWorkdir('/app')
      .setStartCmd('echo "Hello, world!"', waitForTimeout(10_000))

    buildInfo = await Template.build(template, name, defaultTimeoutBuildOpts())
    extra.template_id = buildInfo.templateId
    extra.build_id = buildInfo.buildId
    extra.elapsed_ms = String(Date.now() - startedAt)

    return {
      language: 'js',
      case: 'template_timeout',
      status: 'ok',
      detail: 'default-timeout base-image build succeeded',
      extra,
    }
  } catch (error) {
    const detail = errorDetail(error)
    const message = detail.toLowerCase()
    extra.elapsed_ms = String(Date.now() - startedAt)
    if (message.includes('404 page not found')) {
      return {
        language: 'js',
        case: 'template_timeout',
        status: 'template_api_unavailable',
        detail,
        extra,
      }
    }
    if (message.includes('aborted due to timeout') || message.includes('timeout')) {
      extra.failure_kind = 'timeout'
      return {
        language: 'js',
        case: 'template_timeout',
        status: 'env_blocked',
        detail,
        extra,
      }
    }
    if (message.includes('internal') || message.includes('build error')) {
      extra.failure_kind = 'backend_error'
      return {
        language: 'js',
        case: 'template_timeout',
        status: 'env_blocked',
        detail,
        extra,
      }
    }
    return {
      language: 'js',
      case: 'template_timeout',
      status: 'error',
      detail,
      extra,
    }
  } finally {
    if (buildInfo?.templateId) {
      try {
        await Sandbox.deleteSnapshot(buildInfo.templateId, connectionOpts())
      } catch {}
    }
    rmSync(tmpDir, { recursive: true, force: true })
  }
}

async function runTemplateMethodsCase(): Promise<Result> {
  const tmpDir = mkdtempSync(join(tmpdir(), 'e2b-js-template-methods-'))
  let buildInfo:
    | {
        templateId: string
        buildId: string
      }
    | undefined
  let sandbox: any | undefined
  const extra: Record<string, string> = {
    template_shape:
      'fromBaseImage+runCmd(root)+makeSymlink+copy(symlink-preserve)+copy(symlink-resolve)',
  }

  try {
    writeFileSync(join(tmpDir, 'test.txt'), 'template symlink content\n')
    symlinkSync('test.txt', join(tmpDir, 'link.txt'))

    const template = Template({ fileContextPath: tmpDir })
      .fromBaseImage()
      .runCmd('test "$(whoami)" = "root"', { user: 'root' })
      .makeSymlink('.bashrc', '.bashrc.local')
      .copy('test.txt', '/app/test.txt', { forceUpload: true })
      .copy('link.txt', '/app/link-preserved.txt', { forceUpload: true })
      .copy('link.txt', '/app/link-resolved.txt', {
        forceUpload: true,
        resolveSymlinks: true,
      })
      .runCmd(`test "$(readlink .bashrc.local)" = ".bashrc"`)
      .runCmd(`test "$(readlink /app/link-preserved.txt)" = "test.txt"`)
      .runCmd(`test "$(cat /app/link-preserved.txt)" = "template symlink content"`)
      .runCmd(`test ! -L /app/link-resolved.txt`)
      .runCmd(`test "$(cat /app/link-resolved.txt)" = "template symlink content"`)

    buildInfo = await Template.build(
      template,
      `js-sdk-template-methods-crosscheck-${Date.now()}`,
      buildOpts()
    )
    extra.template_id = buildInfo.templateId
    extra.build_id = buildInfo.buildId

    sandbox = await Sandbox.create(buildInfo.templateId, {
      ...connectionOpts(),
      timeoutMs: 10 * 60_000,
    })
    const result = await sandbox.commands.run(templateMethodsSummaryCommand)
    const summary = String(result.stdout).trim()

    if (summary !== expectedTemplateMethodsSummary) {
      return {
        language: 'js',
        case: 'template_methods',
        status: 'error',
        detail: `unexpected runtime summary:\n${summary}`,
        extra,
      }
    }

    return {
      language: 'js',
      case: 'template_methods',
      status: 'ok',
      detail: 'stable base-image template method summary matched across build and runtime',
      extra,
    }
  } catch (error) {
    const detail = errorDetail(error)
    const message = detail.toLowerCase()
    if (message.includes('404 page not found')) {
      return {
        language: 'js',
        case: 'template_methods',
        status: 'template_api_unavailable',
        detail,
        extra,
      }
    }
    if (
      message.includes('aborted due to timeout') ||
      message.includes('timeout') ||
      message.includes('internal') ||
      message.includes('build error') ||
      message.includes('error waiting for provisioning sandbox')
    ) {
      return {
        language: 'js',
        case: 'template_methods',
        status: 'env_blocked',
        detail,
        extra: { ...extra, failure_kind: 'backend_or_timeout' },
      }
    }
    return {
      language: 'js',
      case: 'template_methods',
      status: 'error',
      detail,
      extra,
    }
  } finally {
    if (sandbox) {
      try {
        await sandbox.kill()
      } catch {}
    }
    if (buildInfo?.templateId) {
      try {
        await Sandbox.deleteSnapshot(buildInfo.templateId, connectionOpts())
      } catch {}
    }
    rmSync(tmpDir, { recursive: true, force: true })
  }
}

async function runConfigHeadersCase(): Promise<Result> {
  let gotTestHeader = ''
  let gotExtraHeader = ''
  let gotUserAgent = ''
  let pauseDetail = ''

  const server = Bun.serve({
    port: 0,
    fetch(request) {
      gotTestHeader = request.headers.get('X-Test') ?? ''
      gotExtraHeader = request.headers.get('X-Extra') ?? ''
      gotUserAgent = request.headers.get('User-Agent') ?? ''
      return new Response(JSON.stringify({ code: 409, message: 'already paused' }), {
        status: 409,
        headers: { 'content-type': 'application/json' },
      })
    },
  })

  try {
    const sandbox = new Sandbox({
      sandboxId: 'sbx-test',
      sandboxDomain: 'sandbox.e2b.dev',
      envdVersion: '0.2.4',
      envdAccessToken: 'tok',
      trafficAccessToken: 'tok',
      apiKey: process.env.E2B_API_KEY,
      apiUrl: `http://127.0.0.1:${server.port}`,
      domain: 'base.e2b.dev',
      requestTimeoutMs: 1000,
      debug: false,
      headers: { 'X-Test': 'base' },
    })

    let paused = false
    try {
      paused = await sandbox.pause({
        headers: { 'X-Extra': '1' },
      })
    } catch (error) {
      pauseDetail = errorDetail(error)
    }

    const extra = {
      x_test: gotTestHeader,
      x_extra: gotExtraHeader,
      user_agent: gotUserAgent,
      paused: String(paused),
      ...(pauseDetail ? { pause_detail: pauseDetail } : {}),
    }
    if (paused !== false) {
      return {
        language: 'js',
        case: 'config_headers',
        status: 'error',
        detail: 'expected pause to return false on 409 conflict',
        extra,
      }
    }
    if (gotTestHeader === 'base' && gotExtraHeader === '1') {
      return {
        language: 'js',
        case: 'config_headers',
        status: 'ok',
        detail: 'pause merged base and per-call headers',
        extra,
      }
    }
    if (gotTestHeader === '' && gotExtraHeader === '1') {
      return {
        language: 'js',
        case: 'config_headers',
        status: 'partial',
        detail: 'pause replaced base headers with per-call headers',
        extra,
      }
    }
    return {
      language: 'js',
      case: 'config_headers',
      status: 'mismatch',
      detail: 'unexpected pause header propagation',
      extra,
    }
  } catch (error) {
    return {
      language: 'js',
      case: 'config_headers',
      status: 'error',
      detail: errorDetail(error),
    }
  } finally {
    server.stop(true)
  }
}

async function runNetworkUpdatePayloadCase(): Promise<Result> {
  const requests: Array<{
    method: string
    path: string
    contentType: string
    body: Record<string, unknown>
  }> = []

  const server = Bun.serve({
    port: 0,
    async fetch(request) {
      const text = await request.text()
      requests.push({
        method: request.method,
        path: new URL(request.url).pathname,
        contentType: request.headers.get('content-type') ?? '',
        body: text ? (JSON.parse(text) as Record<string, unknown>) : {},
      })

      return new Response(null, { status: 204 })
    },
  })

  const opts = {
    apiKey: 'e2b_0000000000000000000000000000000000000000',
    apiUrl: `http://127.0.0.1:${server.port}`,
    domain: 'base.e2b.dev',
    requestTimeoutMs: 1000,
    debug: false,
  }

  try {
    await Sandbox.updateNetwork(
      'sbx-selectors',
      {
        rules: {
          'httpbin.e2b.team': [
            {},
            {
              transform: {
                headers: {
                  'X-Test': 'selector',
                },
              },
            },
          ],
        },
        allowOut: ({ rules }) => [...rules.keys()],
        denyOut: ({ allTraffic }) => [allTraffic],
        allowInternetAccess: false,
      },
      opts
    )

    await Sandbox.updateNetwork(
      'sbx-empty',
      {
        allowOut: [],
        denyOut: [],
        rules: {},
      },
      opts
    )

    if (requests.length !== 2) {
      return {
        language: 'js',
        case: 'network_update_payload',
        status: 'error',
        detail: `expected 2 captured requests, got ${requests.length}`,
      }
    }

    const expectedSelectorBody = {
      allowOut: ['httpbin.e2b.team'],
      allow_internet_access: false,
      denyOut: [ALL_TRAFFIC],
      rules: {
        'httpbin.e2b.team': [
          {},
          {
            transform: {
              headers: {
                'X-Test': 'selector',
              },
            },
          },
        ],
      },
    }

    const selector = requests[0]
    const explicitEmpty = requests[1]
    const expectedExplicitEmptyBody = {
      allowOut: [],
      denyOut: [],
      rules: {},
    }
    const extra = {
      selector_method: selector.method,
      selector_path: selector.path,
      selector_content_type: selector.contentType,
      selector_body: stableStringify(selector.body),
      explicit_empty_method: explicitEmpty.method,
      explicit_empty_path: explicitEmpty.path,
      explicit_empty_content_type: explicitEmpty.contentType,
      explicit_empty_body: stableStringify(explicitEmpty.body),
      explicit_empty_mode:
        stableStringify(explicitEmpty.body) ===
        stableStringify(expectedExplicitEmptyBody)
          ? 'preserved'
          : 'mismatch',
    }

    if (
      selector.method !== 'PUT' ||
      selector.path !== '/sandboxes/sbx-selectors/network'
    ) {
      return {
        language: 'js',
        case: 'network_update_payload',
        status: 'mismatch',
        detail: 'unexpected selector update request target',
        extra,
      }
    }
    if (!selector.contentType.startsWith('application/json')) {
      return {
        language: 'js',
        case: 'network_update_payload',
        status: 'mismatch',
        detail: 'unexpected selector update content-type',
        extra,
      }
    }
    if (stableStringify(selector.body) !== stableStringify(expectedSelectorBody)) {
      return {
        language: 'js',
        case: 'network_update_payload',
        status: 'mismatch',
        detail: 'selector-based update payload did not match expected shape',
        extra,
      }
    }
    if (
      explicitEmpty.method !== 'PUT' ||
      explicitEmpty.path !== '/sandboxes/sbx-empty/network'
    ) {
      return {
        language: 'js',
        case: 'network_update_payload',
        status: 'mismatch',
        detail: 'unexpected explicit-empty update request target',
        extra,
      }
    }
    if (!explicitEmpty.contentType.startsWith('application/json')) {
      return {
        language: 'js',
        case: 'network_update_payload',
        status: 'mismatch',
        detail: 'unexpected explicit-empty update content-type',
        extra,
      }
    }
    if (
      stableStringify(explicitEmpty.body) !==
      stableStringify(expectedExplicitEmptyBody)
    ) {
      return {
        language: 'js',
        case: 'network_update_payload',
        status: 'mismatch',
        detail:
          'explicit-empty update payload did not preserve empty allowOut/denyOut/rules',
        extra,
      }
    }

    return {
      language: 'js',
      case: 'network_update_payload',
      status: 'ok',
      detail: 'captured selector-based and explicit-empty network update payloads locally',
      extra,
    }
  } catch (error) {
    return {
      language: 'js',
      case: 'network_update_payload',
      status: 'error',
      detail: errorDetail(error),
    }
  } finally {
    server.stop(true)
  }
}

async function runTemplateApiPayloadCase(): Promise<Result> {
  const requests: Array<{
    method: string
    path: string
    contentType: string
    body: Record<string, unknown>
  }> = []

  const server = Bun.serve({
    port: 0,
    async fetch(request) {
      const url = new URL(request.url)
      const text = await request.text()
      requests.push({
        method: request.method,
        path: `${url.pathname}${url.search}`,
        contentType: request.headers.get('content-type') ?? '',
        body: text ? (JSON.parse(text) as Record<string, unknown>) : {},
      })

      if (request.method === 'POST' && url.pathname === '/v3/templates') {
        return Response.json({
          templateID: 'tmpl-1',
          buildID: 'bld-1',
          aliases: ['tmpl'],
          names: ['tmpl'],
          public: false,
          tags: ['stable'],
        })
      }
      if (
        request.method === 'POST' &&
        url.pathname === '/v2/templates/tmpl-1/builds/bld-1'
      ) {
        return new Response(null, { status: 200 })
      }
      if (
        request.method === 'GET' &&
        url.pathname === '/templates/aliases/tmpl'
      ) {
        return Response.json({ templateID: 'tmpl-1' })
      }
      if (
        request.method === 'GET' &&
        url.pathname === '/templates/tmpl-1/builds/bld-1/status'
      ) {
        return Response.json({
          templateID: 'tmpl-1',
          buildID: 'bld-1',
          status: 'ready',
          logEntries: [],
          logs: [],
        })
      }
      if (request.method === 'POST' && url.pathname === '/templates/tags') {
        return Response.json({
          buildID: 'bld-1',
          tags: ['stable'],
        })
      }
      if (request.method === 'DELETE' && url.pathname === '/templates/tags') {
        return new Response(null, { status: 204 })
      }
      if (request.method === 'GET' && url.pathname === '/templates/tmpl-1/tags') {
        return Response.json([
          {
            tag: 'stable',
            buildID: 'bld-1',
            createdAt: '2026-05-30T00:00:00Z',
          },
        ])
      }

      return new Response('unexpected path', { status: 404 })
    },
  })

  const opts = {
    apiKey: 'e2b_0000000000000000000000000000000000000000',
    apiUrl: `http://127.0.0.1:${server.port}`,
    domain: 'base.e2b.dev',
    requestTimeoutMs: 1000,
    debug: false,
  }

  try {
    const buildInfo = await Template.buildInBackground(
      Template().fromBaseImage().runCmd('echo hi'),
      'tmpl',
      {
        ...opts,
        tags: ['stable'],
      }
    )
    const exists = await Template.exists('tmpl', opts)
    if (!exists) {
      return {
        language: 'js',
        case: 'template_api_payload',
        status: 'error',
        detail: 'expected Template.exists to return true',
      }
    }
    const status = await Template.getBuildStatus(
      { templateId: buildInfo.templateId, buildId: buildInfo.buildId },
      { ...opts, logsOffset: 3 }
    )
    if (status.status !== 'ready') {
      return {
        language: 'js',
        case: 'template_api_payload',
        status: 'error',
        detail: `unexpected build status ${String(status.status)}`,
      }
    }
    const tagInfo = await Template.assignTags('tmpl:latest', 'stable', opts)
    if (tagInfo.buildId !== 'bld-1' || stableStringify(tagInfo.tags) !== '["stable"]') {
      return {
        language: 'js',
        case: 'template_api_payload',
        status: 'error',
        detail: `unexpected tag info ${stableStringify(tagInfo)}`,
      }
    }
    await Template.removeTags('tmpl', 'stable', opts)
    const tags = await Template.getTags('tmpl-1', opts)
    if (tags.length !== 1 || tags[0].tag !== 'stable' || tags[0].buildId !== 'bld-1') {
      return {
        language: 'js',
        case: 'template_api_payload',
        status: 'error',
        detail: `unexpected tags ${stableStringify(tags)}`,
      }
    }

    if (requests.length !== 7) {
      return {
        language: 'js',
        case: 'template_api_payload',
        status: 'error',
        detail: `expected 7 captured requests, got ${requests.length}`,
      }
    }

    const expected = {
      request_build: {
        method: 'POST',
        path: '/v3/templates',
        contentType: 'application/json',
        body: {
          name: 'tmpl',
          tags: ['stable'],
          cpuCount: 2,
          memoryMB: 1024,
        },
      },
      trigger_build: {
        method: 'POST',
        path: '/v2/templates/tmpl-1/builds/bld-1',
        contentType: 'application/json',
        body: {
          force: false,
          fromImage: 'e2bdev/base',
          steps: [
            {
              type: 'RUN',
              args: ['echo hi'],
              force: false,
            },
          ],
        },
      },
      exists: {
        method: 'GET',
        path: '/templates/aliases/tmpl',
        contentType: '',
        body: {},
      },
      status: {
        method: 'GET',
        path: '/templates/tmpl-1/builds/bld-1/status?logsOffset=3',
        contentType: '',
        body: {},
      },
      assign_tags: {
        method: 'POST',
        path: '/templates/tags',
        contentType: 'application/json',
        body: {
          target: 'tmpl:latest',
          tags: ['stable'],
        },
      },
      remove_tags: {
        method: 'DELETE',
        path: '/templates/tags',
        contentType: 'application/json',
        body: {
          name: 'tmpl',
          tags: ['stable'],
        },
      },
      get_tags: {
        method: 'GET',
        path: '/templates/tmpl-1/tags',
        contentType: '',
        body: {},
      },
    }

    const keys = [
      'request_build',
      'trigger_build',
      'exists',
      'status',
      'assign_tags',
      'remove_tags',
      'get_tags',
    ] as const

    const extra: Record<string, string> = {}
    for (const [index, key] of keys.entries()) {
      const actual = requests[index]
      const want = expected[key]
      extra[`${key}_method`] = actual.method
      extra[`${key}_path`] = actual.path
      extra[`${key}_content_type`] = actual.contentType
      extra[`${key}_body`] = stableStringify(actual.body)

      if (actual.method !== want.method || actual.path !== want.path) {
        return {
          language: 'js',
          case: 'template_api_payload',
          status: 'mismatch',
          detail: `${key} request target mismatch`,
          extra,
        }
      }
      if (want.contentType && !actual.contentType.startsWith(want.contentType)) {
        return {
          language: 'js',
          case: 'template_api_payload',
          status: 'mismatch',
          detail: `${key} content-type mismatch`,
          extra,
        }
      }
      if (stableStringify(actual.body) !== stableStringify(want.body)) {
        return {
          language: 'js',
          case: 'template_api_payload',
          status: 'mismatch',
          detail: `${key} payload mismatch`,
          extra,
        }
      }
    }

    return {
      language: 'js',
      case: 'template_api_payload',
      status: 'ok',
      detail: 'captured template control-plane request shapes locally',
      extra,
    }
  } catch (error) {
    return {
      language: 'js',
      case: 'template_api_payload',
      status: 'error',
      detail: errorDetail(error),
    }
  } finally {
    server.stop(true)
  }
}

async function runMetricsCase(): Promise<Result> {
  const resolved = await resolveSandboxTemplate()
  if ('error' in resolved) {
    return {
      language: 'js',
      case: 'metrics',
      status: 'error',
      detail: resolved.error,
      extra: resolved.extra,
    }
  }

  const extra: Record<string, string> = {
    ...(resolved.extra ?? {}),
    template: resolved.template,
    template_resolution: resolved.detail,
  }

  let sandbox: any | undefined
  try {
    sandbox = await Sandbox.create(resolved.template, {
      ...connectionOpts(),
      timeoutMs: 10 * 60_000,
    })

    const warmup = await sandbox.commands.run(`python3 - <<'PY'
print(sum(range(1000)))
PY`)
    extra.warmup_exit_code = String(warmup.exitCode)

    const deadline = Date.now() + 60_000
    while (Date.now() < deadline) {
      const metrics = await sandbox.getMetrics()
      if (metrics.length > 0) {
        const metric = metrics[0]
        const metricTimestamp = new Date(metric.timestamp)
        const inclusiveStart = new Date(metricTimestamp.getTime() - 2_000)
        const inclusiveEnd = new Date(metricTimestamp.getTime() + 2_000)
        const filtered = await sandbox.getMetrics({
          start: inclusiveStart,
          end: inclusiveEnd,
        })
        const futureStart = new Date(metricTimestamp.getTime() + 24 * 60 * 60 * 1000)
        const futureEnd = new Date(futureStart.getTime() + 2_000)
        const futureFiltered = await sandbox.getMetrics({
          start: futureStart,
          end: futureEnd,
        })
        const rawInclusiveCount = await fetchRawMetricsCount(
          sandbox.sandboxId,
          inclusiveStart,
          inclusiveEnd
        )
        const rawFutureCount = await fetchRawMetricsCount(
          sandbox.sandboxId,
          futureStart,
          futureEnd
        )
        extra.metrics_count = String(metrics.length)
        extra.filtered_count = String(filtered.length)
        extra.future_filtered_count = String(futureFiltered.length)
        extra.raw_inclusive_filtered_count = String(rawInclusiveCount)
        extra.raw_future_filtered_count = String(rawFutureCount)
        extra.metric_timestamp = metricTimestamp.toISOString()
        extra.inclusive_start = inclusiveStart.toISOString()
        extra.inclusive_end = inclusiveEnd.toISOString()
        extra.future_start = futureStart.toISOString()
        extra.future_end = futureEnd.toISOString()
        extra.cpu_count = String(metric.cpuCount)
        extra.cpu_used_pct = String(metric.cpuUsedPct)
        extra.mem_used = String(metric.memUsed)
        extra.mem_total = String(metric.memTotal)
        extra.disk_used = String(metric.diskUsed)
        extra.disk_total = String(metric.diskTotal)
        if (metric.memCache !== undefined) {
          extra.mem_cache = String(metric.memCache)
        }
        if (filtered.length === 0) {
          return {
            language: 'js',
            case: 'metrics',
            status: 'partial',
            detail:
              'metrics returned data, but an inclusive filtered window around the metric timestamp returned zero items',
            extra,
          }
        }
        if (futureFiltered.length !== 0) {
          return {
            language: 'js',
            case: 'metrics',
            status: 'partial',
            detail:
              rawFutureCount === 0
                ? 'future-only filtered metrics window still returned data while the raw control-plane query returned zero rows'
                : 'future-only filtered metrics window still returned data',
            extra,
          }
        }
        return {
          language: 'js',
          case: 'metrics',
          status: 'ok',
          detail: 'metrics available',
          extra,
        }
      }
      await Bun.sleep(500)
    }

    return {
      language: 'js',
      case: 'metrics',
      status: 'env_blocked',
      detail: 'metrics endpoint returned zero points within 60s',
      extra,
    }
  } catch (error) {
    return {
      language: 'js',
      case: 'metrics',
      status: 'error',
      detail: errorDetail(error),
      extra,
    }
  } finally {
    if (sandbox) {
      try {
        await sandbox.kill()
      } catch {}
    }
    if (resolved.extra?.template_source === 'temporary_build') {
      try {
        await Sandbox.deleteSnapshot(resolved.template, connectionOpts())
      } catch {}
    }
  }
}

async function runNetworkRulesCase(): Promise<Result> {
  const resolved = await resolveSandboxTemplate()
  if ('error' in resolved) {
    return {
      language: 'js',
      case: 'network_rules',
      status: 'error',
      detail: resolved.error,
      extra: resolved.extra,
    }
  }

  const extra: Record<string, string> = {
    ...(resolved.extra ?? {}),
    template: resolved.template,
    template_resolution: resolved.detail,
  }

  let sandbox: any | undefined
  try {
    sandbox = await Sandbox.create(resolved.template, {
      ...connectionOpts(),
      timeoutMs: 10 * 60_000,
      network: {
        allowOut: ['httpbin.e2b.team'],
        denyOut: [ALL_TRAFFIC],
        rules: {
          'httpbin.e2b.team': [
            {
              transform: {
                headers: {
                  'X-E2B-Test-Token': 'e2b-transform-value-123',
                },
              },
            },
          ],
        },
      },
    })

    const commandResult = await sandbox.commands.run(
      'curl -sS --max-time 10 https://httpbin.e2b.team/headers'
    )
    const parsed = JSON.parse(String(commandResult.stdout))
    const reflected = parsed.headers?.['X-E2B-Test-Token'] ?? ''
    extra.reflected_header = String(reflected)

    if (reflected !== 'e2b-transform-value-123') {
      return {
        language: 'js',
        case: 'network_rules',
        status: 'env_blocked',
        detail: `network transform is not enforced; reflected header=${JSON.stringify(reflected)}`,
        extra,
      }
    }

    return {
      language: 'js',
      case: 'network_rules',
      status: 'ok',
      detail: 'network transform reflected expected header',
      extra,
    }
  } catch (error) {
    return {
      language: 'js',
      case: 'network_rules',
      status: 'env_blocked',
      detail: errorDetail(error),
      extra,
    }
  } finally {
    if (sandbox) {
      try {
        await sandbox.kill()
      } catch {}
    }
    if (resolved.extra?.template_source === 'temporary_build') {
      try {
        await Sandbox.deleteSnapshot(resolved.template, connectionOpts())
      } catch {}
    }
  }
}

async function runNetworkEgressCase(): Promise<Result> {
  try {
    const exists = await Template.exists('base', connectionOpts())
    if (!exists) {
      return {
        language: 'js',
        case: 'network_egress',
        status: 'template_missing',
        detail: 'base template alias is unavailable',
      }
    }
  } catch (error) {
    return {
      language: 'js',
      case: 'network_egress',
      status: 'error',
      detail: errorDetail(error),
    }
  }

  const extra: Record<string, string> = {
    template: 'base',
    template_source: 'base_alias',
    template_resolution: 'source test alias',
  }

  let firstFailure = ''
  let sandbox: any | undefined

  try {
    sandbox = await Sandbox.create('base', {
      ...connectionOpts(),
      timeoutMs: 10 * 60_000,
      network: {
        denyOut: [ALL_TRAFFIC],
        allowOut: ['1.1.1.1'],
      },
    })
    extra.allow_only_1111 = await runCommandSummary(
      sandbox,
      "curl -s -o /dev/null -w '%{http_code}' https://1.1.1.1"
    )
    extra.allow_only_8888 = await runCommandSummary(
      sandbox,
      'curl --connect-timeout 3 --max-time 5 -Is https://8.8.8.8'
    )
    if (!commandSucceeded(extra.allow_only_1111) && !firstFailure) {
      firstFailure = `allow_only_1111 did not succeed: ${extra.allow_only_1111}`
    }
    if (!commandBlocked(extra.allow_only_8888) && !firstFailure) {
      firstFailure = `allow_only_8888 was not blocked: ${extra.allow_only_8888}`
    }
  } catch (error) {
    return { language: 'js', case: 'network_egress', status: 'error', detail: errorDetail(error), extra }
  } finally {
    if (sandbox) {
      try {
        await sandbox.kill()
      } catch {}
      sandbox = undefined
    }
  }

  try {
    sandbox = await Sandbox.create('base', {
      ...connectionOpts(),
      timeoutMs: 10 * 60_000,
      network: {
        denyOut: ['8.8.8.8'],
      },
    })
    extra.deny_specific_8888 = await runCommandSummary(
      sandbox,
      'curl --connect-timeout 3 --max-time 5 -Is https://8.8.8.8'
    )
    extra.deny_specific_1111 = await runCommandSummary(
      sandbox,
      "curl -s -o /dev/null -w '%{http_code}' https://1.1.1.1"
    )
    if (!commandBlocked(extra.deny_specific_8888) && !firstFailure) {
      firstFailure = `deny_specific_8888 was not blocked: ${extra.deny_specific_8888}`
    }
    if (!commandSucceeded(extra.deny_specific_1111) && !firstFailure) {
      firstFailure = `deny_specific_1111 did not succeed: ${extra.deny_specific_1111}`
    }
  } catch (error) {
    return { language: 'js', case: 'network_egress', status: 'error', detail: errorDetail(error), extra }
  } finally {
    if (sandbox) {
      try {
        await sandbox.kill()
      } catch {}
      sandbox = undefined
    }
  }

  try {
    sandbox = await Sandbox.create('base', {
      ...connectionOpts(),
      timeoutMs: 10 * 60_000,
      network: {
        denyOut: [ALL_TRAFFIC],
        allowOut: ['1.1.1.1', '8.8.8.8'],
      },
    })
    extra.allow_precedence_1111 = await runCommandSummary(
      sandbox,
      "curl -s -o /dev/null -w '%{http_code}' https://1.1.1.1"
    )
    extra.allow_precedence_8888 = await runCommandSummary(
      sandbox,
      "curl -s -o /dev/null -w '%{http_code}' https://8.8.8.8"
    )
    if (!commandSucceeded(extra.allow_precedence_1111) && !firstFailure) {
      firstFailure = `allow_precedence_1111 did not succeed: ${extra.allow_precedence_1111}`
    }
    if (!commandSucceeded(extra.allow_precedence_8888) && !firstFailure) {
      firstFailure = `allow_precedence_8888 did not succeed: ${extra.allow_precedence_8888}`
    }
  } catch (error) {
    return { language: 'js', case: 'network_egress', status: 'error', detail: errorDetail(error), extra }
  } finally {
    if (sandbox) {
      try {
        await sandbox.kill()
      } catch {}
      sandbox = undefined
    }
  }

  try {
    sandbox = await Sandbox.create('base', {
      ...connectionOpts(),
      timeoutMs: 10 * 60_000,
    })
    extra.update_before_8888 = await runCommandSummary(
      sandbox,
      "curl -s -o /dev/null -w '%{http_code}' https://8.8.8.8"
    )
    await sandbox.updateNetwork({ denyOut: ['8.8.8.8'] })
    extra.update_after_deny_8888 = await runCommandSummary(
      sandbox,
      'curl --connect-timeout 3 --max-time 5 -Is https://8.8.8.8'
    )
    extra.update_after_deny_1111 = await runCommandSummary(
      sandbox,
      "curl -s -o /dev/null -w '%{http_code}' https://1.1.1.1"
    )
    if (!commandSucceeded(extra.update_before_8888) && !firstFailure) {
      firstFailure = `update_before_8888 did not succeed: ${extra.update_before_8888}`
    }
    if (!commandBlocked(extra.update_after_deny_8888) && !firstFailure) {
      firstFailure = `update_after_deny_8888 was not blocked: ${extra.update_after_deny_8888}`
    }
    if (!commandSucceeded(extra.update_after_deny_1111) && !firstFailure) {
      firstFailure = `update_after_deny_1111 did not succeed: ${extra.update_after_deny_1111}`
    }
  } catch (error) {
    return { language: 'js', case: 'network_egress', status: 'error', detail: errorDetail(error), extra }
  } finally {
    if (sandbox) {
      try {
        await sandbox.kill()
      } catch {}
      sandbox = undefined
    }
  }

  try {
    sandbox = await Sandbox.create('base', {
      ...connectionOpts(),
      timeoutMs: 10 * 60_000,
      network: {
        denyOut: [ALL_TRAFFIC],
        allowOut: ['1.1.1.1'],
      },
    })
    extra.clear_before_8888 = await runCommandSummary(
      sandbox,
      'curl --connect-timeout 3 --max-time 5 -Is https://8.8.8.8'
    )
    await sandbox.updateNetwork({})
    extra.clear_after_1111 = await runCommandSummary(
      sandbox,
      "curl -s -o /dev/null -w '%{http_code}' https://1.1.1.1"
    )
    extra.clear_after_8888 = await runCommandSummary(
      sandbox,
      "curl -s -o /dev/null -w '%{http_code}' https://8.8.8.8"
    )
    if (!commandBlocked(extra.clear_before_8888) && !firstFailure) {
      firstFailure = `clear_before_8888 was not blocked: ${extra.clear_before_8888}`
    }
    if (!commandSucceeded(extra.clear_after_1111) && !firstFailure) {
      firstFailure = `clear_after_1111 did not succeed: ${extra.clear_after_1111}`
    }
    if (!commandSucceeded(extra.clear_after_8888) && !firstFailure) {
      firstFailure = `clear_after_8888 did not succeed: ${extra.clear_after_8888}`
    }
  } catch (error) {
    return { language: 'js', case: 'network_egress', status: 'error', detail: errorDetail(error), extra }
  } finally {
    if (sandbox) {
      try {
        await sandbox.kill()
      } catch {}
    }
  }

  if (firstFailure) {
    return {
      language: 'js',
      case: 'network_egress',
      status: 'env_blocked',
      detail: firstFailure,
      extra,
    }
  }

  return {
    language: 'js',
    case: 'network_egress',
    status: 'ok',
    detail: 'source-like network egress expectations matched',
    extra,
  }
}

async function waitForFinalBuildStatus(templateId: string, buildId: string) {
  const deadline = Date.now() + 30 * 60_000
  while (Date.now() < deadline) {
    const status = await Template.getBuildStatus({ templateId, buildId }, connectionOpts())
    if (status.status !== 'building' && status.status !== 'waiting') {
      return status
    }
    await Bun.sleep(5_000)
  }
  throw new Error('timed out waiting for final ubuntu build status')
}

async function runNumpyVector(sandbox: any) {
  const result = await sandbox.commands.run(numpyRandomCommand)
  return String(result.stdout).trim()
}

async function resolveSandboxTemplate():
  Promise<
    | { template: string; detail: string; extra?: Record<string, string> }
    | { error: string; extra?: Record<string, string> }
  > {
  for (const key of [
    'E2B_TEST_TEMPLATE',
    'E2B_INTEGRATION_TEMPLATE',
    'E2B_TEMPLATE',
    'E2B_SANDBOX_TEMPLATE',
  ]) {
    const value = process.env[key]
    if (value) {
      return {
        template: value,
        detail: 'from env',
        extra: { template_source: key },
      }
    }
  }

  const extra: Record<string, string> = {}

  try {
    const exists = await Template.exists('base', connectionOpts())
    if (exists) {
      return {
        template: 'base',
        detail: 'from base alias',
        extra: { template_source: 'base_alias' },
      }
    }
  } catch (error) {
    extra.base_exists_error = errorDetail(error)
  }

  try {
    const paginator = Sandbox.list({
      ...connectionOpts(),
      limit: 10,
    })
    const items = await paginator.nextItems()
    const inferred = items.find((item: any) => item.templateId)
    if (inferred?.templateId) {
      return {
        template: inferred.templateId,
        detail: 'inferred from existing sandbox',
        extra: {
          ...extra,
          template_source: 'inferred_from_list',
          inferred_sandbox_id: inferred.sandboxId,
        },
      }
    }
  } catch (error) {
    extra.list_error = errorDetail(error)
  }

  try {
    const name = `js-sdk-metrics-crosscheck-${Date.now()}`
    const info = await Template.build(Template().fromBaseImage(), name, buildOpts())
    return {
      template: info.templateId,
      detail: 'temporary base-image build',
      extra: {
        ...extra,
        template_source: 'temporary_build',
        template_id: info.templateId,
      },
    }
  } catch (error) {
    return {
      error: errorDetail(error),
      extra,
    }
  }
}

async function fetchRawMetricsCount(
  sandboxId: string,
  start?: Date,
  end?: Date
): Promise<number> {
  const conn = connectionOpts()
  const baseUrl =
    conn.apiUrl || (conn.domain ? `https://api.${conn.domain}` : '')
  if (!baseUrl) {
    throw new Error('missing API URL/domain for raw metrics request')
  }

  const url = new URL(`/sandboxes/${sandboxId}/metrics`, baseUrl)
  if (start) {
    url.searchParams.set('start', String(Math.round(start.getTime() / 1000)))
  }
  if (end) {
    url.searchParams.set('end', String(Math.round(end.getTime() / 1000)))
  }

  const response = await fetch(url, {
    headers: {
      ...(conn.apiKey ? { 'X-API-KEY': conn.apiKey } : {}),
      ...(conn.accessToken ? { Authorization: `Bearer ${conn.accessToken}` } : {}),
    },
  })
  const text = await response.text()
  if (!response.ok) {
    throw new Error(`raw metrics request failed: status=${response.status} body=${text}`)
  }
  const data = JSON.parse(text)
  if (!Array.isArray(data)) {
    throw new Error(`raw metrics response was not an array: ${text}`)
  }
  return data.length
}

function classifyCommandError(
  caseName: 'claude' | 'claude_derived' | 'randomness',
  error: unknown
): Result {
  const detail = errorDetail(error)
  const message = detail.toLowerCase()
  if (
    message.includes('no module named') ||
    message.includes('numpy') ||
    message.includes('python3: not found')
  ) {
    return {
      language: 'js',
      case: caseName,
      status: 'env_blocked',
      detail,
    }
  }
  return {
    language: 'js',
    case: caseName,
    status: 'error',
    detail,
  }
}

async function runCommandSummary(sandbox: any, command: string) {
  try {
    const result = await sandbox.commands.run(command)
    return `ok:${String(result.stdout).trim()}`
  } catch (error) {
    const text = errorDetail(error)
    const match = text.match(/code\s+(\d+)/i) || text.match(/exit status[: ]+(\d+)/i)
    if (match) {
      return `exit:${match[1]}`
    }
    return `error:${text}`
  }
}

function commandSucceeded(summary: string) {
  return summary.startsWith('ok:')
}

function commandBlocked(summary: string) {
  return summary.startsWith('exit:')
}

function classifyVolumeError(error: unknown): Result {
  const detail = errorDetail(error)
  const message = detail.toLowerCase()
  if (message.includes('path /multi-file-dir not found') || message.includes('not found')) {
    return {
      language: 'js',
      case: 'volume',
      status: 'env_blocked',
      detail,
    }
  }
  return {
    language: 'js',
    case: 'volume',
    status: 'error',
    detail,
  }
}

function toErrorResult(caseName: Result['case'], error: unknown): Result {
  return {
    language: 'js',
    case: caseName,
    status: 'error',
    detail: errorDetail(error),
  }
}

function mergeExtra(
  base: Record<string, string> | undefined,
  extra: Record<string, string>
) {
  if (!base) {
    return extra
  }
  return {
    ...base,
    ...extra,
  }
}

function errorDetail(error: unknown): string {
  const err = error as {
    stdout?: string
    stderr?: string
    cause?: unknown
  }
  return [String(error), err?.stdout, err?.stderr]
    .filter((value): value is string => Boolean(value))
    .join('\n')
}

function stableStringify(value: unknown): string {
  return JSON.stringify(sortForStableStringify(value))
}

function sortForStableStringify(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(sortForStableStringify)
  }
  if (value !== null && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>)
        .sort(([left], [right]) => left.localeCompare(right))
        .map(([key, nested]) => [key, sortForStableStringify(nested)])
    )
  }
  return value
}

function restoreEnv(key: string, value: string | undefined) {
  if (value === undefined) {
    delete process.env[key]
    return
  }
  process.env[key] = value
}
