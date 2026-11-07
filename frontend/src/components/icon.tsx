import type { ImageAsset } from 'nsh'
import { Image } from 'nsh'
import AlertCircle from '@/assets/icons/AlertCircle.svg'
import AlertTriangle from '@/assets/icons/AlertTriangle.svg'
import ArrowLeft from '@/assets/icons/ArrowLeft.svg'
import ArrowRight from '@/assets/icons/ArrowRight.svg'
import Check from '@/assets/icons/Check.svg'
import CheckCircle2 from '@/assets/icons/CheckCircle2.svg'
import ChevronDown from '@/assets/icons/ChevronDown.svg'
import ChevronLeft from '@/assets/icons/ChevronLeft.svg'
import ChevronRight from '@/assets/icons/ChevronRight.svg'
import CirclePlus from '@/assets/icons/CirclePlus.svg'
import Clock3 from '@/assets/icons/Clock3.svg'
import Download from '@/assets/icons/Download.svg'
import Ellipsis from '@/assets/icons/Ellipsis.svg'
import ExternalLink from '@/assets/icons/ExternalLink.svg'
import Headphones from '@/assets/icons/Headphones.svg'
import Heart from '@/assets/icons/Heart.svg'
import HelpCircle from '@/assets/icons/HelpCircle.svg'
import History from '@/assets/icons/History.svg'
import House from '@/assets/icons/House.svg'
import Info from '@/assets/icons/Info.svg'
import ListMusic from '@/assets/icons/ListMusic.svg'
import LockKeyhole from '@/assets/icons/LockKeyhole.svg'
import MessageCircle from '@/assets/icons/MessageCircle.svg'
import MoreHorizontal from '@/assets/icons/MoreHorizontal.svg'
import Music2 from '@/assets/icons/Music2.svg'
import Play from '@/assets/icons/Play.svg'
import Plus from '@/assets/icons/Plus.svg'
import Search from '@/assets/icons/Search.svg'
import Send from '@/assets/icons/Send.svg'
import Settings from '@/assets/icons/Settings.svg'
import Share2 from '@/assets/icons/Share2.svg'
import SkipForward from '@/assets/icons/SkipForward.svg'
import Sparkles from '@/assets/icons/Sparkles.svg'
import Trash2 from '@/assets/icons/Trash2.svg'
import User from '@/assets/icons/User.svg'
import Users from '@/assets/icons/Users.svg'
import Volume2 from '@/assets/icons/Volume2.svg'
import VolumeX from '@/assets/icons/VolumeX.svg'
import Wifi from '@/assets/icons/Wifi.svg'
import WifiOff from '@/assets/icons/WifiOff.svg'
import X from '@/assets/icons/X.svg'
import { muted } from '@/theme'

export const icons = {
  AlertCircle,
  AlertTriangle,
  ArrowLeft,
  ArrowRight,
  Check,
  CheckCircle2,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  CirclePlus,
  Clock3,
  Download,
  Ellipsis,
  ExternalLink,
  Headphones,
  Heart,
  HelpCircle,
  History,
  House,
  Info,
  ListMusic,
  LockKeyhole,
  MessageCircle,
  MoreHorizontal,
  Music2,
  Play,
  Plus,
  Search,
  Send,
  Settings,
  Share2,
  SkipForward,
  Sparkles,
  Trash2,
  User,
  Users,
  Volume2,
  VolumeX,
  Wifi,
  WifiOff,
  X,
} satisfies Record<string, ImageAsset>

export type IconName = keyof typeof icons

export function Icon(props: { name: IconName, size?: number, color?: string }) {
  return (
    <Image
      src={icons[props.name]}
      accessibilityElementsHidden
      style={{ width: props.size ?? 20, height: props.size ?? 20, flexShrink: 0, tintColor: props.color ?? muted, objectFit: 'contain' }}
    />
  )
}
