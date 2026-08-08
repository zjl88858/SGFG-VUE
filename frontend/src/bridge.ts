import { Events } from '@wailsio/runtime'
import { ProxyService } from '../bindings/sgfg-ng'
import type { ConnectOptions } from '../bindings/sgfg-ng/internal/proxy/models'
import type { StatusInfo } from '../bindings/sgfg-ng/internal/proxy/models'

export interface LogEntry {
  level: string
  msg: string
}

export const api = {
  selectTransport: (t: string) => ProxyService.SelectTransport(t),
  pingSelected: () => ProxyService.PingSelected(),
  setOptions: (o: ConnectOptions) => ProxyService.SetOptions(o),
  options: () => ProxyService.Options(),
  connect: () => ProxyService.Connect(),
  disconnect: () => ProxyService.Disconnect(),
  startRelay: (port: number, password: string) => ProxyService.StartRelay(port, password),
  stopRelay: () => ProxyService.StopRelay(),
  generatePassword: () => ProxyService.GeneratePassword(),
  status: () => ProxyService.Status(),
  version: () => ProxyService.Version(),
  openOptions: () => ProxyService.OpenOptions(),
  closeOptions: () => ProxyService.CloseOptions(),
}

export function onLog(cb: (e: LogEntry) => void) {
  Events.On('sgfg:log', (ev: any) => {
    const d = ev?.data ?? ev
    if (d && typeof d === 'object') cb({ level: String(d.level ?? 'info'), msg: String(d.msg ?? '') })
  })
}

export interface ScanEvent {
  attempt: number
  max: number
  host: string
}

export function onScan(cb: (e: ScanEvent) => void) {
  Events.On('sgfg:scan', (ev: any) => {
    const d = ev?.data ?? ev
    if (d && typeof d === 'object') {
      cb({ attempt: Number(d.attempt ?? 0), max: Number(d.max ?? 3), host: String(d.host ?? '') })
    }
  })
}

export function onState(cb: (s: StatusInfo) => void) {
  Events.On('sgfg:state', (ev: any) => {
    const d = ev?.data ?? ev
    if (d && typeof d === 'object') cb(d as StatusInfo)
  })
}

export function onOptions(cb: (o: ConnectOptions) => void) {
  Events.On('sgfg:options', (ev: any) => {
    const d = ev?.data ?? ev
    if (d && typeof d === 'object') cb(d as ConnectOptions)
  })
}
