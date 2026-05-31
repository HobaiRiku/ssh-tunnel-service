import { computed, ref } from 'vue'

export type Locale = 'en' | 'zh-CN'

const messages = {
  en: {
    app: {
      brand: 'SSH Tunnel Service',
      tunnels: 'Tunnels',
      remotes: 'Remotes',
      keys: 'Keys',
      english: 'EN',
      chinese: '中文',
    },
    common: {
      add: 'Add',
      save: 'Save',
      cancel: 'Cancel',
      close: 'Close',
      edit: 'Edit',
      delete: 'Delete',
      refresh: 'Refresh',
      start: 'Start',
      stop: 'Stop',
      ssh: 'SSH Command',
      upload: 'Upload',
      systemDefault: 'System default',
      optional: 'Optional',
      loading: 'Loading…',
      noSelection: 'None',
      topology: 'Topology',
      table: 'Table',
      running: 'Running',
      stopped: 'Stopped',
      error: 'Error',
    },
    topology: {
      noRemotesTitle: 'No remotes yet',
      noRemotesSub: 'Create a remote and then add tunnels to view the topology here.',
      noTunnelsTitle: 'No tunnels yet',
      noTunnelsSub: 'Add a tunnel to render the topology and manage it directly from this view.',
      clickHint: 'Click a tunnel or target node to open the action panel directly.',
      selected: 'Selected tunnel',
      local: 'Local',
      bind: 'Bind',
      target: 'Target',
      remote: 'Remote',
      state: 'State',
      allRemotes: 'All',
    },
    tunnels: {
      title: 'Tunnels',
      addTitle: 'Add Tunnel',
      editTitle: 'Edit Tunnel',
      added: 'Tunnel added',
      updated: 'Tunnel updated',
      deleted: 'Tunnel deleted',
      startRequested: 'Start requested',
      stopped: 'Tunnel stopped',
      deleteConfirm: 'Delete this tunnel?',
      commandTitle: 'Equivalent SSH Command · {name}',
      commandHelp: 'SSH arguments the service will pass to ssh for this tunnel. Note: POSIX shell rendering — may not be directly runnable on non-POSIX shells.',
      actionTitle: 'Tunnel · {name}',
      fields: {
        id: 'ID',
        name: 'Name',
        remote: 'Remote',
        direction: 'Direction',
        bindAddress: 'Bind Address',
        bindPort: 'Bind Port',
        targetHost: 'Target Host',
        targetPort: 'Target Port',
        autoStart: 'Auto Start',
        description: 'Description',
        state: 'State',
      },
      columns: {
        name: 'Name',
        remote: 'Remote',
        direction: 'Direction',
        bind: 'Bind',
        target: 'Target',
        state: 'State',
        actions: 'Actions',
      },
      direction: {
        localTitle: 'Local forward',
        localSummary: 'Access a remote service from this machine.',
        localBindMeaning: 'Bind = address/port this machine listens on (usually 127.0.0.1).',
        localTargetMeaning: 'Target = remote-side service host:port reached through the SSH server.',
        remoteTitle: 'Remote forward',
        remoteSummary: 'Expose a local service on the remote machine.',
        remoteBindMeaning: 'Bind = address/port exposed on the remote side.',
        remoteTargetMeaning: 'Target = local-side service host:port on this machine.',
      },
    },
    remotes: {
      title: 'Remotes',
      addTitle: 'Add Remote',
      editTitle: 'Edit Remote',
      added: 'Remote added',
      updated: 'Remote updated',
      deleted: 'Remote deleted',
      deleteConfirm: 'Delete this remote?',
      fields: {
        id: 'ID',
        name: 'Name',
        host: 'Host',
        port: 'Port',
        user: 'User',
        key: 'Key',
        description: 'Description',
      },
      columns: {
        name: 'Name',
        host: 'Host',
        port: 'Port',
        user: 'User',
        key: 'Key',
        description: 'Description',
        actions: 'Actions',
      },
    },
    keys: {
      title: 'Keys',
      addTitle: 'Add Key',
      editTitle: 'Edit Key',
      added: 'Key added',
      updated: 'Key updated',
      deleted: 'Key deleted',
      deleteConfirm: 'Delete this key?',
      uploadHint: 'Paste the private key content, or upload an existing key file. The key will be stored under the runtime home directory.',
      replaceHint: 'Leave key material empty to keep the existing stored file unchanged.',
      fields: {
        id: 'ID',
        name: 'Name',
        fileName: 'File Name',
        privateKey: 'Private Key',
        upload: 'Upload File',
        description: 'Description',
      },
      columns: {
        name: 'Name',
        file: 'File',
        description: 'Description',
        actions: 'Actions',
      },
    },
  },
  'zh-CN': {
    app: {
      brand: 'SSH 隧道服务',
      tunnels: '隧道',
      remotes: '远端',
      keys: '密钥',
      english: 'EN',
      chinese: '中文',
    },
    common: {
      add: '新增',
      save: '保存',
      cancel: '取消',
      close: '关闭',
      edit: '编辑',
      delete: '删除',
      refresh: '刷新',
      start: '启动',
      stop: '停止',
      ssh: 'SSH 命令',
      upload: '上传',
      systemDefault: '系统默认',
      optional: '可选',
      loading: '加载中…',
      noSelection: '无',
      topology: '拓扑',
      table: '表格',
      running: '运行中',
      stopped: '已停止',
      error: '错误',
    },
    topology: {
      noRemotesTitle: '还没有远端',
      noRemotesSub: '先创建远端，再添加隧道即可在这里查看拓扑。',
      noTunnelsTitle: '还没有隧道',
      noTunnelsSub: '添加隧道后即可在这里查看并直接操作拓扑。',
      clickHint: '点击隧道或目标节点可直接打开操作面板。',
      selected: '当前选中',
      local: '本机',
      bind: '监听',
      target: '目标',
      remote: '远端',
      state: '状态',
      allRemotes: '全部',
    },
    tunnels: {
      title: '隧道',
      addTitle: '新增隧道',
      editTitle: '编辑隧道',
      added: '隧道已新增',
      updated: '隧道已更新',
      deleted: '隧道已删除',
      startRequested: '已请求启动',
      stopped: '隧道已停止',
      deleteConfirm: '确认删除该隧道？',
      commandTitle: '等效 SSH 命令 · {name}',
      commandHelp: '这是服务为该隧道传给 ssh 的参数。注意：按 POSIX shell 格式展示，在非 POSIX shell 中可能无法直接运行。',
      actionTitle: '隧道 · {name}',
      fields: {
        id: 'ID',
        name: '名称',
        remote: '远端',
        direction: '方向',
        bindAddress: '监听地址',
        bindPort: '监听端口',
        targetHost: '目标主机',
        targetPort: '目标端口',
        autoStart: '自动启动',
        description: '描述',
        state: '状态',
      },
      columns: {
        name: '名称',
        remote: '远端',
        direction: '方向',
        bind: '监听',
        target: '目标',
        state: '状态',
        actions: '操作',
      },
      direction: {
        localTitle: '本地转发',
        localSummary: '从本机访问远端服务。',
        localBindMeaning: '监听 = 当前机器监听的地址/端口（通常为 127.0.0.1）。',
        localTargetMeaning: '目标 = 经由 SSH 远端访问到的远端服务主机和端口。',
        remoteTitle: '远程转发',
        remoteSummary: '把本地服务暴露到远端机器。',
        remoteBindMeaning: '监听 = 暴露在远端侧的地址和端口。',
        remoteTargetMeaning: '目标 = 当前机器上的本地服务主机和端口。',
      },
    },
    remotes: {
      title: '远端',
      addTitle: '新增远端',
      editTitle: '编辑远端',
      added: '远端已新增',
      updated: '远端已更新',
      deleted: '远端已删除',
      deleteConfirm: '确认删除该远端？',
      fields: {
        id: 'ID',
        name: '名称',
        host: '主机',
        port: '端口',
        user: '用户',
        key: '密钥',
        description: '描述',
      },
      columns: {
        name: '名称',
        host: '主机',
        port: '端口',
        user: '用户',
        key: '密钥',
        description: '描述',
        actions: '操作',
      },
    },
    keys: {
      title: '密钥',
      addTitle: '新增密钥',
      editTitle: '编辑密钥',
      added: '密钥已新增',
      updated: '密钥已更新',
      deleted: '密钥已删除',
      deleteConfirm: '确认删除该密钥？',
      uploadHint: '可直接粘贴私钥内容，或上传已有私钥文件；密钥会存储在运行配置目录下。',
      replaceHint: '编辑时如果不填写新的密钥内容，将继续使用当前已存储的文件。',
      fields: {
        id: 'ID',
        name: '名称',
        fileName: '文件名',
        privateKey: '私钥内容',
        upload: '上传文件',
        description: '描述',
      },
      columns: {
        name: '名称',
        file: '文件',
        description: '描述',
        actions: '操作',
      },
    },
  },
}

const storageKey = 'ssh-tunnel-service.locale'
const browserLocale = resolveBrowserLocale()
const initialLocale = parseLocale(typeof localStorage !== 'undefined' ? localStorage.getItem(storageKey) : null)
const locale = ref<Locale>(initialLocale === 'zh-CN' || initialLocale === 'en' ? initialLocale : browserLocale)

function parseLocale(value: string | null): Locale | null {
  return value === 'zh-CN' || value === 'en' ? value : null
}

function resolveBrowserLocale(): Locale {
  if (typeof navigator === 'undefined') {
    return 'en'
  }
  const normalized = navigator.language.toLowerCase()
  if (normalized === 'zh-cn' || normalized.startsWith('zh-hans')) {
    return 'zh-CN'
  }
  return 'en'
}

function lookup(localeCode: Locale, key: string): string {
  const parts = key.split('.')
  let value: unknown = messages[localeCode]
  for (const part of parts) {
    if (typeof value !== 'object' || value === null || !Object.prototype.hasOwnProperty.call(value, part)) {
      return key
    }
    value = (value as Record<string, unknown>)[part]
  }
  return typeof value === 'string' ? value : key
}

export function setLocale(next: Locale) {
  locale.value = next
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(storageKey, next)
  }
}

export function useI18n() {
  const t = (key: string, params?: Record<string, string | number>) => {
    let value = lookup(locale.value, key)
    if (params) {
      for (const [name, replacement] of Object.entries(params)) {
        value = value.replaceAll(`{${name}}`, String(replacement))
      }
    }
    return value
  }

  return {
    locale: computed(() => locale.value),
    setLocale,
    t,
  }
}
