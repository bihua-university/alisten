import { Modal } from './ui'

export default function HelpModal({ onClose }: { onClose: () => void }) {
  return (
    <Modal title="使用帮助" onClose={onClose}>
      <div className="help-content text-ui-base">
        <section>
          <h3 className="text-ui-lg">一起听歌</h3>
          <p>
            加入房间后会自动同步播放。浏览器需要交互时，点击「开始播放」。本地暂停只影响你自己的播放，恢复时会重新同步。
          </p>
        </section>
        <section>
          <h3 className="text-ui-lg">搜索与点歌</h3>
          <p>
            点击顶部搜索，选择网易云音乐或 QQ
            音乐，点击歌曲旁的加号加入队列。歌单搜索结果可打开查看；也可以输入
            *歌单ID 或 @用户ID。
          </p>
        </section>
        <section>
          <h3 className="text-ui-lg">队列与切歌</h3>
          <p>
            在播放列表中点赞、移除歌曲。播放器的下一首按钮会发起切歌投票，达到房间要求后切换歌曲。
          </p>
        </section>
        <section>
          <h3 className="text-ui-lg">聊天与分享</h3>
          <p>
            在聊天栏输入消息并回车发送。支持表情字符；支持快捷命令的房间可发送「点歌
            歌名」或「点歌
            音频链接」。分享按钮会复制房间链接，密码需单独告诉朋友。
          </p>
        </section>
        <section>
          <h3 className="text-ui-lg">移动端与省电</h3>
          <p>
            使用底部导航切换聆听、播放列表和聊天。设置中的省电或极简模式会减少动画、模糊和阴影。界面支持系统的减少动态效果设置。
          </p>
        </section>
        <section>
          <h3 className="text-ui-lg">安装与更新</h3>
          <p>
            在支持的浏览器中可将应用安装到主屏幕。新版本准备完成后会提示更新；房间连接和音频播放需要网络。
          </p>
        </section>
      </div>
    </Modal>
  )
}
