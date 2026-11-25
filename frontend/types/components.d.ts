declare module 'vue' {
  export interface GlobalComponents {
    Icon: (typeof import('@novelworkbench/components'))['Icon']
  }
}

export {}
