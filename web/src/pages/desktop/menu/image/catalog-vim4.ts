export type CatalogImage = {
  id: string;
  name: string;
  source: string;
  size: number;
  cdrom: boolean;
  disk?: boolean;
};

export const mediaSources: Record<string, string> = {
  shared: 'NanoKVM 镜像',
  jetkvm: 'JetKVM 镜像',
  onekvm: 'One-KVM 镜像',
  pikvm: 'PiKVM 镜像',
  data: '共享数据盘'
};

export function catalogFile(entry: CatalogImage) {
  return entry.source === 'shared'
    ? `/var/lib/vim4-kvm/images/${entry.id.slice('shared/'.length)}`
    : entry.id;
}
