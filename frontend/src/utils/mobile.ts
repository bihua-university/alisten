/**
 * 移动端适配工具函数
 */

// 可滚动区域选择器
export const MOBILE_SCROLL_SELECTORS = `
  .lyrics-container,
  .overflow-y-auto,
  .overflow-auto,
  .modal-scroll,
  .mobile-chat-scroll,
  .smooth-scroll,
  .scrollbar-hide,
  .custom-scrollbar,
  [data-scrollable="true"],
  [class*="scroll"]
`.replace(/\s+/g, ' ').trim()

/** 与布局实际使用的 Tailwind md=768px 断点保持一致 */
const MOBILE_BREAKPOINT_QUERY = '(max-width: 767px)'
/** 粗指针：以手指为主要输入。可识别 UA 已伪装为 macOS 的 iPad 与折叠屏 */
const COARSE_POINTER_QUERY = '(pointer: coarse)'

/**
 * 检测是否为移动端。
 *
 * 组合「窄视口」与「触摸优先」两个信号，替代原先的 UA 正则嗅探：
 * - 窄视口：桌面窗口拖窄时同样成立，与 CSS 断点行为保持一致
 * - 粗指针：UA 嗅探识别不了的 iPad（iPadOS 13+ 上报为 macOS）可以正确命中
 */
export function isMobileDevice(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function')
    return false

  return window.matchMedia(MOBILE_BREAKPOINT_QUERY).matches
    || window.matchMedia(COARSE_POINTER_QUERY).matches
}

/**
 * 订阅移动端判定变化，返回取消订阅函数。
 *
 * 用媒体查询替代 resize + innerWidth 轮询：旋屏、折叠屏展开、窗口缩放都会可靠触发，
 * 且不必在每次 resize 中重复计算。
 */
export function onMobileDeviceChange(handler: (isMobile: boolean) => void): () => void {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function')
    return () => {}

  const queries = [
    window.matchMedia(MOBILE_BREAKPOINT_QUERY),
    window.matchMedia(COARSE_POINTER_QUERY),
  ]
  const listener = () => handler(isMobileDevice())

  queries.forEach(query => query.addEventListener('change', listener))
  return () => queries.forEach(query => query.removeEventListener('change', listener))
}

let viewportRafId: number | null = null
let lastViewportHeight = -1

/**
 * 设置视口高度 CSS 变量。
 *
 * 现代浏览器的高度已由 CSS 的 `100dvh` 接管，这里只是旧浏览器的兜底。
 * 因此用 requestAnimationFrame 合并高频 resize 调用，并跳过数值未变化的写入，
 * 避免移动端滚动（地址栏收放会持续触发 resize）时反复重排整页。
 */
export function setViewportHeight(): void {
  if (viewportRafId !== null)
    return

  viewportRafId = requestAnimationFrame(() => {
    viewportRafId = null
    const height = window.innerHeight
    if (height === lastViewportHeight)
      return
    lastViewportHeight = height
    document.documentElement.style.setProperty('--vh', `${height * 0.01}px`)
  })
}

function resolveTargetElement(target: EventTarget | null): Element | null {
  if (target instanceof Element)
    return target

  if (target instanceof Node)
    return target.parentElement

  return null
}

/**
 * 检查元素是否在允许滚动的区域内
 */
export function isScrollableElement(target: EventTarget | null): Element | null {
  const element = resolveTargetElement(target)
  if (!element)
    return null

  // 首先检查精确的滚动选择器
  const exactMatch = element.closest(MOBILE_SCROLL_SELECTORS)
  if (exactMatch) {
    return exactMatch
  }

  // 检查元素本身或父元素是否有滚动相关的类名
  let current: Element | null = element
  while (current && current !== document.body) {
    const classList = current.className || ''

    // 检查是否包含滚动相关的类名
    if (
      classList.includes('overflow-y')
      || classList.includes('overflow-auto')
      || classList.includes('scroll')
      || (classList.includes('flex-1') && current.querySelector('[class*="overflow"]'))
    ) {
      return current
    }

    current = current.parentElement
  }

  return null
}

/**
 * 检查是否在模态框中
 */
export function isInModal(target: EventTarget | null): boolean {
  const element = resolveTargetElement(target)
  if (!element)
    return false

  return !!element.closest('.fixed, .modal, .dialog, .popup, [class*="z-"]')
}

/**
 * 检查是否在应用内
 */
export function isInApp(target: EventTarget | null): boolean {
  const element = resolveTargetElement(target)
  if (!element)
    return false

  return !!element.closest('#root')
}

/**
 * 防止页面级滚动的事件处理器
 */
export function createPreventScrollHandler() {
  return function preventScroll(e: Event) {
    // 阻止页面级别的滚动
    if (e.target === document.body || e.target === document.documentElement) {
      e.preventDefault()
      return
    }

    const target = e.target
    const scrollableElement = isScrollableElement(target)

    // 如果在可滚动区域内，总是允许滚动
    if (scrollableElement) {
      return
    }

    // 检查是否在模态框内，如果在模态框内则更宽松地处理
    const inModal = isInModal(target)
    if (inModal) {
      return
    }

    // 只有当不在可滚动区域且不在模态框内时才阻止滚动
    const inApp = isInApp(target)
    if (!inApp) {
      e.preventDefault()
      e.stopPropagation()
    }
  }
}

/**
 * 防止触摸滚动的事件处理器
 */
export function createPreventTouchMoveHandler() {
  return function preventTouchMove(e: TouchEvent) {
    const target = e.target

    // 检查是否在允许滚动的区域内
    const scrollableElement = isScrollableElement(target)

    if (!scrollableElement) {
      // 检查是否在模态框内，如果在模态框内但没有找到滚动区域，可能是嵌套的情况
      const inModal = isInModal(target)
      if (!inModal) {
        e.preventDefault()
        e.stopPropagation()
      }
    }

    // 在滚动区域内，总是允许滚动，让浏览器自然处理边界
  }
}

/**
 * 防止多点触摸手势的事件处理器
 */
export function createPreventTouchStartHandler() {
  return function preventTouchStart(e: TouchEvent) {
    const target = e.target
    const scrollableElement = isScrollableElement(target)
    if (!scrollableElement && e.touches.length > 1) {
      e.preventDefault()
    }
  }
}
