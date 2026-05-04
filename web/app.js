const $ = (id) => document.getElementById(id);

const DOM = {
  loginPanel: $('loginPanel'),
  appPanel: $('appPanel'),
  loginForm: $('loginForm'),
  username: $('username'),
  password: $('password'),
  connectBtn: $('connectBtn'),
  registerBtn: $('registerBtn'),
  loginAvatarUpload: $('loginAvatarUpload'),
  loginAvatarPreview: $('loginAvatarPreview'),
  settingsAvatarUpload: $('settingsAvatarUpload'),
  settingsAvatarPreview: $('settingsAvatarPreview'),
  headerAvatarBtn: $('headerAvatarBtn'),
  chatAvatarBtn: $('chatAvatarBtn'),
  profileAvatarBtn: $('profileAvatarBtn'),
  profileDialogAvatar: $('profileDialogAvatar'),
  userNameDisplay: $('userNameDisplay'),
  userStatus: $('userStatus'),
  profileUsername: $('profileUsername'),
  profileSignature: $('profileSignature'),
  profileSince: $('profileSince'),
  chatTitle: $('chatTitle'),
  chatSubtitle: $('chatSubtitle'),
  messageList: $('messageList'),
  messageInput: $('messageInput'),
  sendBtn: $('sendBtn'),
  toUser: $('toUser'),
  toGroup: $('toGroup'),
  privateRow: $('privateRow'),
  groupRow: $('groupRow'),
  targetBar: $('targetBar'),
  onlineList: $('onlineList'),
  recentList: $('recentList'),
  groupSessionList: $('groupSessionList'),
  groupSessionCount: $('groupSessionCount'),
  sideOnlineList: $('sideOnlineList'),
  sideGroupList: $('sideGroupList'),
  wsStatus: $('wsStatus'),
  memberMetric: $('memberMetric'),
  todayMetric: $('todayMetric'),
  themeQuickBtn: $('themeQuickBtn'),
  quickAddFriendBtn: $('quickAddFriendBtn'),
  quickCreateGroupBtn: $('quickCreateGroupBtn'),
  quickUploadAvatarBtn: $('quickUploadAvatarBtn'),
  imageToolBtn: $('imageToolBtn'),
  fileToolBtn: $('fileToolBtn'),
  onlineCount: $('onlineCount'),
  friendCount: $('friendCount'),
  statFriends: $('statFriends'),
  globalSearchInput: $('globalSearchInput'),
  newMessageBtn: $('newMessageBtn'),
  refreshOnlineBtn: $('refreshOnlineBtn'),
  openAddFriendBtn: $('openAddFriendBtn'),
  loadHistoryBtn: $('loadHistoryBtn'),
  logoutBtn: $('logoutBtn'),
  profileDialog: $('profileDialog'),
  groupsDialog: $('groupsDialog'),
  contactsDialog: $('contactsDialog'),
  settingsDialog: $('settingsDialog'),
  discoverDialog: $('discoverDialog'),
  discoverPublicBtn: $('discoverPublicBtn'),
  discoverOnlineBtn: $('discoverOnlineBtn'),
  discoverFriendsBtn: $('discoverFriendsBtn'),
  discoverGroupsBtn: $('discoverGroupsBtn'),
  discoverRefreshBtn: $('discoverRefreshBtn'),
  discoverAddFriendBtn: $('discoverAddFriendBtn'),
  discoverOnlineCount: $('discoverOnlineCount'),
  discoverFriendCount: $('discoverFriendCount'),
  discoverGroupCount: $('discoverGroupCount'),
  discoverOnlineList: $('discoverOnlineList'),
  thoughtComposerAvatar: $('thoughtComposerAvatar'),
  thoughtInput: $('thoughtInput'),
  publishThoughtBtn: $('publishThoughtBtn'),
  thoughtFeed: $('thoughtFeed'),
  viewProfileBtn: $('viewProfileBtn'),
  saveProfileBtn: $('saveProfileBtn'),
  saveAvatarBtn: $('saveAvatarBtn'),
  profileNameInput: $('profileNameInput'),
  profileGenderInput: $('profileGenderInput'),
  profileSignatureInput: $('profileSignatureInput'),
  myGroupsList: $('myGroupsList'),
  newGroupName: $('newGroupName'),
  createGroupBtn: $('createGroupBtn'),
  joinGroupBtn: $('joinGroupBtn'),
  inviteMemberName: $('inviteMemberName'),
  inviteGroupName: $('inviteGroupName'),
  inviteGroupBtn: $('inviteGroupBtn'),
  friendsList: $('friendsList'),
  friendNameInput: $('friendNameInput'),
  addFriendBtn: $('addFriendBtn'),
  reloadFriendsBtn: $('reloadFriendsBtn'),
  darkModeToggle: $('darkModeToggle'),
  soundToggle: $('soundToggle'),
  fontSizeSlider: $('fontSizeSlider'),
  fontSizeValue: $('fontSizeValue'),
  statMessages: $('statMessages'),
  statGroups: $('statGroups'),
  toastHost: $('toastHost'),
  topicTags: $('topicTags')
};

const state = {
  token: '',
  username: '',
  avatar: '🐱',
  gender: '',
  signature: '',
  mode: 'public',
  selectedPeer: '',
  selectedPeerAvatar: '🐱',
  groups: [],
  friends: [],
  onlineUsers: [],
  thoughts: [],
  messages: 0,
  uploadedAvatar: '',
  ws: null,
  wsReconnectTimer: null,
  wsRetries: 0
};

function escapeHtml(value) {
  const s = String(value ?? '');
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

function avatarMarkup(avatar, label = '头像') {
  if (avatar && (avatar.startsWith('data:') || avatar.startsWith('/uploads/'))) return `<img src="${escapeHtml(avatar)}" alt="${label}" />`;
  return `<span>${escapeHtml(avatar || '🐱')}</span>`;
}

function setAvatar(el, avatar) {
  if (el) el.innerHTML = avatarMarkup(avatar);
}

function toast(message, type = 'info') {
  const el = document.createElement('div');
  el.className = `toast toast-${type}`;
  el.textContent = message;
  DOM.toastHost.appendChild(el);
  while (DOM.toastHost.children.length > 5) {
    DOM.toastHost.children[0].remove();
  }
  window.setTimeout(() => el.remove(), 2800);
}

async function api(endpoint, options = {}) {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 15000);
  try {
    const headers = options.headers || {};
    headers['Content-Type'] = 'application/json';
    if (state.token) {
      headers['Authorization'] = 'Bearer ' + state.token;
    }
    const body = options.body ? JSON.stringify(options.body) : undefined;
    const response = await fetch(endpoint, {
      ...options,
      body,
      headers,
      signal: controller.signal,
    });
    if (!response.ok) {
      const text = await response.text().catch(() => '');
      throw new Error(text || `HTTP ${response.status}`);
    }
    const text = await response.text();
    return text ? JSON.parse(text) : {};
  } catch (err) {
    if (err.name === 'AbortError') throw new Error('请求超时');
    throw err;
  } finally {
    clearTimeout(timeoutId);
  }
}

function showApp() {
  DOM.loginPanel.hidden = true;
  DOM.appPanel.hidden = false;
}

function showLogin() {
  DOM.loginPanel.hidden = false;
  DOM.appPanel.hidden = true;
}

function nowLabel(dateLike) {
  const date = dateLike ? new Date(dateLike) : new Date();
  if (Number.isNaN(date.getTime())) return new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

function setMessageCount(nextValue) {
  state.messages = nextValue;
  DOM.statMessages.textContent = state.messages.toLocaleString();
  if (DOM.todayMetric) DOM.todayMetric.textContent = `今日消息 ${state.messages.toLocaleString()}`;
}

function setConnectionStatus(connected) {
  if (!DOM.wsStatus) return;
  DOM.wsStatus.classList.toggle('online', connected);
  DOM.wsStatus.classList.toggle('offline', !connected);
  DOM.wsStatus.querySelector('b').textContent = connected ? '已连接' : '未连接';
}

function renderSidePanels() {
  if (DOM.memberMetric) DOM.memberMetric.textContent = `成员 ${state.onlineUsers.length.toLocaleString()}`;
  if (DOM.sideOnlineList) {
    const online = state.onlineUsers.filter((user) => user.name !== state.username).slice(0, 6);
    DOM.sideOnlineList.innerHTML = online.length
      ? online.map((user) => `<li><span>${escapeHtml(user.name)}</span><b>在线</b></li>`).join('')
      : '<li><span>暂无其他在线用户</span><b>等待中</b></li>';
  }
  if (DOM.sideGroupList) {
    DOM.sideGroupList.innerHTML = state.groups.length
      ? state.groups.slice(0, 6).map((name) => `<li><span>${escapeHtml(name)}</span><b>群组</b></li>`).join('')
      : '<li><span>暂无群组</span><b>创建一个</b></li>';
  }
}

function emptyState(title, detail = '', tag = 'div') {
  return `
    <${tag} class="empty-state">
      <strong>${escapeHtml(title)}</strong>
      ${detail ? `<span>${escapeHtml(detail)}</span>` : ''}
    </${tag}>
  `;
}

function resetMessages(title = '暂无消息', detail = '发送第一条消息开始聊天。') {
  setMessageCount(0);
  DOM.messageList.innerHTML = `<div class="day-pill">今天</div>${emptyState(title, detail)}`;
}

function addMessage({ text, type = 'incoming', avatar = '', time = nowLabel(), system = false, sender = '' }) {
  DOM.messageList.querySelector('.empty-state')?.remove();
  if (system) {
    const el = document.createElement('div');
    el.className = 'system-msg';
    el.textContent = text;
    DOM.messageList.appendChild(el);
    const el2 = DOM.messageList;
    const isNearBottom = el2.scrollHeight - el2.scrollTop - el2.clientHeight < 200;
    if (isNearBottom) {
      el2.scrollTop = el2.scrollHeight;
    }
    return;
  }

  const row = document.createElement('div');
  row.className = `message-row ${type}`;
  const profileTarget = sender || (type === 'outgoing' ? state.username : state.selectedPeer);
  row.innerHTML = `
    <button class="mini-avatar" type="button" title="查看资料">${avatarMarkup(avatar)}</button>
    <article class="bubble">
      <p>${escapeHtml(text)}</p>
      <time>${escapeHtml(time)}</time>
    </article>
  `;
  row.querySelector('.mini-avatar').addEventListener('click', () => {
    if (profileTarget) openProfile(profileTarget);
  });
  DOM.messageList.appendChild(row);
  const el = DOM.messageList;
  const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 200;
  if (isNearBottom) {
    el.scrollTop = el.scrollHeight;
  }
}

function updateChatTitle() {
  if (state.mode === 'private') {
    const peer = DOM.toUser.value.trim() || state.selectedPeer || '选择联系人';
    DOM.chatTitle.textContent = peer;
    DOM.chatSubtitle.textContent = peer === '选择联系人' ? '从在线列表或联系人中选择用户' : '私聊会话';
    return;
  }
  if (state.mode === 'group') {
    const group = DOM.toGroup.value.trim() || '群组会话';
    DOM.chatTitle.textContent = group;
    DOM.chatSubtitle.textContent = '群组消息频道';
    return;
  }
  DOM.chatTitle.textContent = '公共聊天室';
  DOM.chatSubtitle.textContent = '所有在线用户可见';
}

function setMode(nextMode) {
  state.mode = nextMode;
  document.querySelectorAll('[data-mode]').forEach((btn) => btn.classList.toggle('active', btn.dataset.mode === nextMode));
  document.querySelectorAll('[data-mode-jump]').forEach((btn) => btn.classList.toggle('active', btn.dataset.modeJump === nextMode));
  DOM.privateRow.style.display = nextMode === 'private' ? 'flex' : 'none';
  DOM.groupRow.style.display = nextMode === 'group' ? 'flex' : 'none';
  DOM.targetBar.classList.toggle('visible', nextMode !== 'public');
  if (nextMode === 'public') setAvatar(DOM.chatAvatarBtn, '🐾');
  updateChatTitle();
}

function selectPeer(name, avatar) {
  state.selectedPeer = name;
  state.selectedPeerAvatar = avatar || '🐱';
  DOM.toUser.value = name;
  setMode('private');
  setAvatar(DOM.chatAvatarBtn, state.selectedPeerAvatar);
  document.querySelectorAll('.session-card').forEach((item) => {
    item.classList.toggle('active', item.dataset.name === name.toLowerCase());
  });
  loadHistory('private', name).catch((err) => toast(err.message, 'error'));
}

function sessionItem(session) {
  const li = document.createElement('li');
  li.className = 'session-card';
  li.dataset.name = session.name.toLowerCase();
  li.dataset.filter = session.filter || 'online';
  li.innerHTML = `
    <button class="session-avatar" type="button" title="查看资料">${avatarMarkup(session.avatar)}</button>
    <div class="session-info">
      <div class="session-head">
        <span class="session-name">${escapeHtml(session.name)}${session.friend ? '<span class="pin">★</span>' : ''}</span>
        <span class="session-time">${escapeHtml(session.time || '')}</span>
      </div>
      <p class="session-preview">${escapeHtml(session.text || '')}</p>
    </div>
    ${session.online ? '<span class="online-dot"></span>' : '<span></span>'}
  `;
  li.querySelector('.session-avatar').addEventListener('click', (event) => {
    event.stopPropagation();
    openProfile(session.name);
  });
  li.addEventListener('click', () => selectPeer(session.name, session.avatar));
  return li;
}

function groupSessionItem(name) {
  const li = document.createElement('li');
  li.className = 'session-card group-session';
  li.dataset.name = name.toLowerCase();
  li.dataset.filter = 'group';
  li.innerHTML = `
    <button class="session-avatar" type="button" title="进入群组"><span>群</span></button>
    <div class="session-info">
      <div class="session-head">
        <span class="session-name">${escapeHtml(name)}<span class="group-tag">Group</span></span>
        <span class="session-time">群聊</span>
      </div>
      <p class="session-preview">进入群组频道，继续闪闪发光的讨论。</p>
    </div>
    <span class="badge">G</span>
  `;
  li.addEventListener('click', () => {
    DOM.toGroup.value = name;
    DOM.inviteGroupName.value = name;
    setMode('group');
    document.querySelectorAll('.session-card').forEach((item) => {
      item.classList.toggle('active', item === li);
    });
    loadHistory('group', name).catch((err) => toast(err.message, 'error'));
  });
  return li;
}

function renderGroupSessions() {
  if (!DOM.groupSessionList) return;
  if (DOM.groupSessionCount) DOM.groupSessionCount.textContent = state.groups.length.toLocaleString();
  DOM.groupSessionList.innerHTML = '';
  if (state.groups.length === 0) {
    DOM.groupSessionList.innerHTML = emptyState('暂无群组', '创建或加入群组后会出现在这里。', 'li');
    return;
  }
  state.groups.forEach((name) => DOM.groupSessionList.appendChild(groupSessionItem(name)));
}

function renderSessionList() {
  const online = state.onlineUsers.filter((user) => user.name !== state.username);
  const friendsByName = new Map(state.friends.map((friend) => [friend.name, friend]));
  DOM.onlineCount.textContent = online.length.toLocaleString();
  DOM.friendCount.textContent = state.friends.length.toLocaleString();
  DOM.statFriends.textContent = state.friends.length.toLocaleString();
  renderSidePanels();

  DOM.onlineList.innerHTML = '';
  if (online.length === 0) {
    DOM.onlineList.innerHTML = emptyState('暂无在线联系人', '让另一个用户登录后会出现在这里。', 'li');
  } else {
    online.forEach((user) => {
      DOM.onlineList.appendChild(sessionItem({
        name: user.name,
        avatar: user.avatar,
        text: friendsByName.has(user.name) ? '好友在线，点击开始私聊。' : '在线用户，点击开始私聊。',
        time: '在线',
        online: true,
        friend: friendsByName.has(user.name),
        filter: friendsByName.has(user.name) ? 'friends' : 'online'
      }));
    });
  }

  DOM.recentList.innerHTML = '';
  if (state.friends.length === 0) {
    DOM.recentList.innerHTML = emptyState('暂无好友', '点击“添加好友”输入用户名即可添加。', 'li');
  } else {
    state.friends.forEach((friend) => {
      const isOnline = state.onlineUsers.some((user) => user.name === friend.name);
      DOM.recentList.appendChild(sessionItem({
        name: friend.name,
        avatar: friend.avatar,
        text: isOnline ? '在线，点击发起私聊。' : '离线，仍可查看历史消息。',
        time: isOnline ? '在线' : '离线',
        online: isOnline,
        friend: true,
        filter: 'friends'
      }));
    });
  }

  applySearchFilter();
}

async function refreshOnline() {
  const data = await api('/api/online');
  state.onlineUsers = data.online || [];
  renderSessionList();
}

async function refreshFriends() {
  if (!state.token) return;
  const data = await api('/api/friends');
  state.friends = data.friends || [];
  renderSessionList();
}

async function refreshGroups() {
  if (!state.token) return;
  const data = await api('/api/groups');
  state.groups = data.groups || [];
  DOM.statGroups.textContent = state.groups.length.toLocaleString();
  renderGroups();
  renderGroupSessions();
  renderSidePanels();
}

function renderGroups() {
  DOM.myGroupsList.innerHTML = '';
  if (state.groups.length === 0) {
    DOM.myGroupsList.innerHTML = '<li><span>暂无群组</span><small>创建或加入群组后会显示在这里。</small></li>';
    return;
  }
  state.groups.forEach((name) => {
    const li = document.createElement('li');
    li.innerHTML = `
      <span>☷ ${escapeHtml(name)}</span>
      <div class="row-actions">
        <button class="secondary-btn" type="button" data-action="enter">进入</button>
        <button class="secondary-btn danger-text" type="button" data-action="leave">离开</button>
      </div>
    `;
    li.querySelector('[data-action="enter"]').addEventListener('click', () => {
      DOM.toGroup.value = name;
      DOM.inviteGroupName.value = name;
      setMode('group');
      DOM.groupsDialog.close();
      loadHistory('group', name).catch((err) => toast(err.message, 'error'));
    });
    li.querySelector('[data-action="leave"]').addEventListener('click', () => leaveGroup(name));
    DOM.myGroupsList.appendChild(li);
  });
}

function renderDiscover() {
  const online = state.onlineUsers.filter((user) => user.name !== state.username);
  DOM.discoverOnlineCount.textContent = `${online.length} 人在线`;
  DOM.discoverFriendCount.textContent = `${state.friends.length} 位好友`;
  DOM.discoverGroupCount.textContent = `${state.groups.length} 个群组`;
  setAvatar(DOM.thoughtComposerAvatar, state.avatar);
  renderThoughts();
  DOM.discoverOnlineList.innerHTML = '';

  if (online.length === 0) {
    DOM.discoverOnlineList.innerHTML = '<li><span>暂无其他在线用户</span><small>让另一个账号登录后会显示在这里。</small></li>';
    return;
  }

  const friendsByName = new Map(state.friends.map((friend) => [friend.name, friend]));
  online.forEach((user) => {
    const li = document.createElement('li');
    const isFriend = friendsByName.has(user.name);
    li.innerHTML = `
      <span class="friend-line">${avatarMarkup(user.avatar)} ${escapeHtml(user.name)}${isFriend ? ' ★' : ''}</span>
      <div class="row-actions">
        <button class="secondary-btn" type="button" data-action="profile">资料</button>
        <button class="secondary-btn" type="button" data-action="chat">私聊</button>
        ${isFriend ? '' : '<button class="secondary-btn" type="button" data-action="add">加好友</button>'}
      </div>
    `;
    li.querySelector('[data-action="profile"]').addEventListener('click', () => openProfile(user.name));
    li.querySelector('[data-action="chat"]').addEventListener('click', () => {
      DOM.discoverDialog.close();
      selectPeer(user.name, user.avatar);
    });
    li.querySelector('[data-action="add"]')?.addEventListener('click', () => addFriend(user.name).catch((err) => toast(err.message, 'error')));
    DOM.discoverOnlineList.appendChild(li);
  });
}

function thoughtStorageKey() {
  return 'nekochat:thoughts:v1';
}

function loadThoughts() {
  try {
    const value = localStorage.getItem(thoughtStorageKey());
    state.thoughts = value ? JSON.parse(value) : [];
    if (!Array.isArray(state.thoughts)) state.thoughts = [];
  } catch (_) {
    state.thoughts = [];
  }
}

function saveThoughts() {
  const compact = state.thoughts.slice(0, 80).map((thought) => ({
    ...thought,
    avatar: thought.avatar?.startsWith('data:') ? '' : thought.avatar
  }));
  try {
    localStorage.setItem(thoughtStorageKey(), JSON.stringify(compact));
  } catch (_) {
    toast('想法本地缓存空间不足，已保留当前页面内容', 'error');
  }
}

function formatThoughtTime(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '刚刚';
  const delta = Date.now() - date.getTime();
  if (delta < 60 * 1000) return '刚刚';
  if (delta < 60 * 60 * 1000) return `${Math.floor(delta / 60000)} 分钟前`;
  if (delta < 24 * 60 * 60 * 1000) return `${Math.floor(delta / 3600000)} 小时前`;
  return date.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' });
}

function visibleThoughts() {
  const visibleNames = new Set([state.username, ...state.friends.map((friend) => friend.name)]);
  return state.thoughts.filter((thought) => visibleNames.has(thought.author));
}

function renderThoughts() {
  DOM.thoughtFeed.innerHTML = '';
  const friendMap = new Map(state.friends.map((friend) => [friend.name, friend]));
  const items = visibleThoughts();
  if (items.length === 0) {
    DOM.thoughtFeed.innerHTML = emptyState('还没有好友动态', '发布一条想法，或者添加好友后查看他们的动态。', 'li');
    return;
  }

  items.forEach((thought) => {
    const friend = friendMap.get(thought.author);
    const avatar = thought.author === state.username ? state.avatar : (friend?.avatar || thought.avatar || '🐱');
    const liked = (thought.likedBy || []).includes(state.username);
    const li = document.createElement('li');
    li.className = 'thought-card';
    li.dataset.id = thought.id;
    const tags = thought.tags && thought.tags.length ? thought.tags : (thought.mood ? [thought.mood] : []);
    li.innerHTML = `
      <header class="thought-head">
        <button class="thought-avatar" type="button" data-action="profile">${avatarMarkup(avatar)}</button>
        <div>
          <strong>${escapeHtml(thought.author)}</strong>
          <span>${escapeHtml(formatThoughtTime(thought.createdAt))}${tags.length ? ` · ${tags.map(t => '#' + t).join(' ')}` : ''}</span>
        </div>
      </header>
      <p class="thought-content">${escapeHtml(thought.content)}</p>
      <div class="thought-actions">
        <button class="secondary-btn" type="button" data-action="like">${liked ? '已赞' : '点赞'} ${Number(thought.likes || 0)}</button>
        ${thought.author === state.username
          ? '<button class="secondary-btn" type="button" data-action="profile">我的资料</button>'
          : '<button class="secondary-btn" type="button" data-action="chat">私聊</button>'}
      </div>
      <div class="thought-comments">
        ${(thought.comments || []).map((comment) => `<p><b>${escapeHtml(comment.author)}：</b>${escapeHtml(comment.content)}</p>`).join('')}
      </div>
      <div class="thought-comment-box">
        <input type="text" placeholder="写评论..." data-role="comment-input" />
        <button class="secondary-btn" type="button" data-action="comment">评论</button>
      </div>
    `;
    DOM.thoughtFeed.appendChild(li);
  });
}

function publishThought() {
  const content = DOM.thoughtInput.value.trim();
  if (!content) return toast('先写一点想法再发布', 'error');
  const selectedTags = Array.from(document.querySelectorAll('.topic-tag.active')).map((btn) => btn.dataset.tag);
  state.thoughts.unshift({
    id: `${Date.now()}-${Math.random().toString(16).slice(2)}`,
    author: state.username,
    avatar: state.avatar?.startsWith('data:') ? '' : state.avatar,
    tags: selectedTags,
    content,
    likes: 0,
    likedBy: [],
    comments: [],
    createdAt: new Date().toISOString()
  });
  DOM.thoughtInput.value = '';
  document.querySelectorAll('.topic-tag.active').forEach((btn) => btn.classList.remove('active'));
  saveThoughts();
  renderDiscover();
  toast('想法已发布');
}

function updateThought(id, updater) {
  const thought = state.thoughts.find((item) => item.id === id);
  if (!thought) return;
  updater(thought);
  saveThoughts();
  renderDiscover();
}

async function openDiscover() {
  await Promise.allSettled([refreshOnline(), refreshFriends(), refreshGroups()]);
  loadThoughts();
  renderDiscover();
  if (!DOM.discoverDialog.open) DOM.discoverDialog.showModal();
}

async function createOrJoinGroup(action) {
  const groupName = DOM.newGroupName.value.trim();
  if (!groupName) return toast('请输入群组名', 'error');
  await api('/api/group', { method: 'POST', body: { action, groupName } });
  DOM.newGroupName.value = '';
  await refreshGroups();
  toast(action === 'create' ? '群组已创建' : '已加入群组');
}

async function inviteToGroup() {
  const memberName = DOM.inviteMemberName.value.trim();
  const groupName = DOM.inviteGroupName.value.trim() || DOM.toGroup.value.trim();
  if (!memberName || !groupName) return toast('请输入成员用户名和群组名', 'error');
  await api('/api/group', { method: 'POST', body: { action: 'invite', groupName, memberName } });
  DOM.inviteMemberName.value = '';
  toast('已邀请成员加入群组');
}

async function leaveGroup(groupName) {
  await api('/api/group', { method: 'POST', body: { action: 'leave', groupName } });
  await refreshGroups();
  toast('已离开群组');
}

async function loadHistory(type = state.mode, target = '') {
  if (!state.token) return;
  const params = new URLSearchParams({ type, limit: '300' });
  if (type === 'private') params.set('peer', target || DOM.toUser.value.trim());
  if (type === 'group') params.set('group', target || DOM.toGroup.value.trim());
  if ((type === 'private' && !params.get('peer')) || (type === 'group' && !params.get('group'))) {
    return toast(type === 'private' ? '请先选择私聊对象' : '请先选择群组', 'error');
  }

  const data = await api(`/api/history?${params.toString()}`);
  const history = data.history || [];
  resetMessages('暂无历史消息', '这里会显示真实发送和接收的消息。');
  if (history.length === 0) return;

  DOM.messageList.querySelector('.empty-state')?.remove();
  history.reverse().forEach((item) => {
    const mine = item.from === state.username;
    const prefix = item.type === 'group' ? `[${item.group}] ${item.from}: ` : item.type === 'private' ? `${item.from}: ` : `${item.from}: `;
    addMessage({
      text: `${prefix}${item.content}`,
      type: mine ? 'outgoing' : 'incoming',
      avatar: mine ? state.avatar : (item.avatar || state.selectedPeerAvatar || '🐱'),
      time: nowLabel(item.created_at),
      sender: item.from
    });
  });
}

async function login() {
  const username = DOM.username.value.trim();
  const password = DOM.password.value.trim();
  if (!username || !password) return toast('请输入用户名和密码', 'error');

  const data = await api('/api/login', { method: 'POST', body: { username, password } });
  if (!data || !data.token) throw new Error('登录响应异常，缺少 token');
  state.token = data.token;
  state.username = username;
  state.avatar = data.avatar || '🐱';
  state.signature = data.signature || '把简单的事情做漂亮 🌸';
  state.gender = data.gender || '';

  DOM.userNameDisplay.textContent = username;
  DOM.userStatus.textContent = '在线';
  DOM.profileUsername.textContent = `@${username}`;
  DOM.profileSignature.textContent = state.signature || '把简单的事情做漂亮 🌸';
  DOM.profileSince.textContent = new Date().toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', year: 'numeric' });
  [DOM.headerAvatarBtn, DOM.chatAvatarBtn, DOM.profileDialogAvatar].forEach((el) => setAvatar(el, state.avatar));

  showApp();
  setMode('public');
  resetMessages('暂无消息', '公共频道会显示真实聊天内容。');
  await Promise.allSettled([refreshOnline(), refreshFriends(), refreshGroups()]);
  await loadHistory('public');
  openWS();
}

async function register() {
  const username = DOM.username.value.trim();
  const password = DOM.password.value.trim();
  if (!username || !password) return toast('请输入用户名和密码', 'error');
  if (password.length < 6) return toast('注册密码至少 6 位', 'error');
  await api('/api/register', {
    method: 'POST',
    body: { username, password, avatar: state.uploadedAvatar || '🐱' }
  });
  state.uploadedAvatar = '';
  DOM.loginAvatarPreview.innerHTML = '📷';
  toast('注册成功，可以登录了');
}

function openWS() {
  try { state.ws?.close(); } catch (_) {}
  setConnectionStatus(false);
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const ws = new WebSocket(`${protocol}//${location.host}/api/ws`);
  state.ws = ws;

  ws.onopen = () => {
    // Send auth as first message after connection established
    ws.send(JSON.stringify({ type: 'auth', token: state.token }));
  };

  ws.onclose = () => {
    if (state.ws === ws) {
      state.ws = null;
      setConnectionStatus(false);
      // Auto reconnect
      if (state.token) {
        const delay = Math.min(2 ** (state.wsRetries || 0) * 1000, 30000);
        state.wsRetries = (state.wsRetries || 0) + 1;
        state.wsReconnectTimer = setTimeout(() => openWS(), delay);
      }
    }
  };

  ws.onerror = () => {
    // WebSocket closes itself on error, onclose handles reconnection
  };

  ws.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data);
      // Handle auth response
      if (data.type === 'auth_ok') {
        if (state.ws === ws) {
          setConnectionStatus(true);
          state.wsRetries = 0;
        }
        return;
      }
      switch (data.type) {
        case 'message':
          if (data.from === state.username) return;
          const parsed = parseIncomingSender(data.content);
          addMessage({
            text: data.content,
            avatar: data.avatar || '🐱',
            sender: data.from || parsed || '',
            time: data.time
          });
          break;
        case 'system':
          addMessage({ text: data.content, system: true });
          refreshOnline().catch(() => {});
          break;
        case 'error':
          toast(data.content, 'error');
          break;
        case 'sent':
          addMessage({
            text: data.content,
            type: 'outgoing',
            avatar: state.avatar,
            sender: state.username,
            time: data.time
          });
          break;
      }
    } catch (e) {
      console.error('ws message parse error', e);
    }
  };
}

async function sendMessage() {
  const text = DOM.messageInput.value.trim();
  if (!text) return;
  if (state.mode === 'private') {
    const to = DOM.toUser.value.trim();
    if (!to) return toast('请选择私聊对象', 'error');
    if (state.ws?.readyState === WebSocket.OPEN) {
      state.ws.send(JSON.stringify({ type: 'send', mode: 'private', to, message: text }));
      DOM.messageInput.value = '';
    } else {
      toast('连接未就绪，消息未发送', 'error');
    }
  } else if (state.mode === 'group') {
    const to = DOM.toGroup.value.trim();
    if (!to) return toast('请输入群组名', 'error');
    if (state.ws?.readyState === WebSocket.OPEN) {
      state.ws.send(JSON.stringify({ type: 'send', mode: 'group', to, message: text }));
      DOM.messageInput.value = '';
    } else {
      toast('连接未就绪，消息未发送', 'error');
    }
  } else {
    if (state.ws?.readyState === WebSocket.OPEN) {
      state.ws.send(JSON.stringify({ type: 'send', mode: 'public', message: text }));
      DOM.messageInput.value = '';
    } else {
      toast('连接未就绪，消息未发送', 'error');
    }
  }
}

function parseIncomingSender(text) {
  const privateMatch = text.match(/^\[私聊\]\s*([^:：]+)[:：]/);
  if (privateMatch) return privateMatch[1].trim();
  const groupMatch = text.match(/^\[群聊[^\]]*\]\s*([^:：]+)[:：]/);
  if (groupMatch) return groupMatch[1].trim();
  const webMatch = text.match(/^\[WEB\]\s*([^:：]+)[:：]/);
  if (webMatch) return webMatch[1].trim();
  const plainMatch = text.match(/^([^:：\]]+)[:：]/);
  return plainMatch ? plainMatch[1].trim() : '';
}

async function logout() {
  if (state.wsReconnectTimer) {
    clearTimeout(state.wsReconnectTimer);
    state.wsReconnectTimer = null;
  }
  try { state.ws?.close(); } catch (_) {}
  state.ws = null;
  setConnectionStatus(false);
  if (state.token) {
    try { await api('/api/logout', { method: 'POST', body: {} }); } catch (_) {}
  }
  Object.assign(state, {
    token: '', username: '', avatar: '🐱',
    selectedPeer: '', selectedPeerAvatar: '🐱',
    groups: [], friends: [], onlineUsers: [], messages: 0,
    uploadedAvatar: null, gender: '', signature: '',
    thoughts: [], wsReconnectTimer: null, wsRetries: 0
  });
  showLogin();
}

function applySearchFilter(filter = document.querySelector('.tab.active')?.dataset.filter || 'all') {
  const keyword = DOM.globalSearchInput.value.trim().toLowerCase();
  document.querySelectorAll('.session-card').forEach((item) => {
    const matchesKeyword = !keyword || item.dataset.name.includes(keyword);
    const matchesFilter = filter === 'all' || item.dataset.filter === filter || (filter === 'online' && item.querySelector('.online-dot'));
    item.style.display = matchesKeyword && matchesFilter ? 'grid' : 'none';
  });
}

function bindAvatarInput(input, preview) {
  input.addEventListener('change', () => {
    const file = input.files?.[0];
    if (!file) return;
    if (file.size > 2 * 1024 * 1024) return toast('图片不能超过 2MB', 'error');
    if (!['image/jpeg', 'image/png', 'image/gif', 'image/webp'].includes(file.type)) return toast('仅支持 JPG / PNG / GIF / WebP', 'error');
    const reader = new FileReader();
    reader.onload = () => {
      state.uploadedAvatar = String(reader.result || '');
      preview.innerHTML = `<img src="${escapeHtml(state.uploadedAvatar)}" alt="预览" />`;
    };
    reader.readAsDataURL(file);
  });
  preview.addEventListener('click', () => input.click());
}

async function openProfile(targetUsername = state.username) {
  if (!state.token) return;
  try {
    const target = targetUsername || state.username;
    const data = await api(`/api/profile?user=${encodeURIComponent(target)}`);
    const isSelf = data.isSelf !== false;
    if (isSelf) {
      state.avatar = data.avatar || state.avatar;
      state.signature = data.signature || '';
      state.gender = data.gender || '';
    }
    DOM.profileDialog.querySelector('h2').textContent = isSelf ? '个人简介' : '用户简介';
    DOM.profileNameInput.value = data.username || target;
    DOM.profileGenderInput.value = data.gender || '';
    DOM.profileSignatureInput.value = data.signature || '';
    DOM.profileNameInput.disabled = !isSelf;
    DOM.profileGenderInput.disabled = !isSelf;
    DOM.profileSignatureInput.disabled = !isSelf;
  DOM.settingsAvatarPreview.parentElement.style.display = isSelf ? 'grid' : 'none';
    DOM.saveAvatarBtn.style.display = isSelf ? '' : 'none';
    DOM.saveProfileBtn.style.display = isSelf ? '' : 'none';
    setAvatar(DOM.profileDialogAvatar, data.avatar || '🐱');
    DOM.profileDialog.showModal();
  } catch (err) {
    toast(err.message, 'error');
  }
}

async function saveProfile() {
  const nextName = DOM.profileNameInput.value.trim();
  const gender = DOM.profileGenderInput.value.trim();
  const signature = DOM.profileSignatureInput.value.trim();
  if (!nextName) return toast('昵称不能为空', 'error');

  if (nextName !== state.username) {
    await api('/api/rename', { method: 'POST', body: { new: nextName } });
    state.username = nextName;
    DOM.userNameDisplay.textContent = nextName;
    DOM.profileUsername.textContent = `@${nextName}`;
  }
  await api('/api/profile', { method: 'POST', body: { gender, signature } });
  state.gender = gender;
  state.signature = signature;
  DOM.profileSignature.textContent = signature || '把简单的事情做漂亮 🌸';
  toast('个人简介已保存');
  DOM.profileDialog.close();
}

async function saveAvatar() {
  if (!state.uploadedAvatar) return toast('请先选择头像', 'error');
  const data = await api('/api/avatar', { method: 'POST', body: { avatar: state.uploadedAvatar } });
  state.avatar = data?.avatar || state.uploadedAvatar;
  state.uploadedAvatar = '';
  [DOM.headerAvatarBtn, DOM.chatAvatarBtn, DOM.profileDialogAvatar].forEach((el) => setAvatar(el, state.avatar));
  DOM.settingsAvatarPreview.innerHTML = '📷';
  toast('头像已保存');
}

function renderFriendsDialog() {
  DOM.friendsList.innerHTML = '';
  if (state.friends.length === 0) {
    DOM.friendsList.innerHTML = '<li><span>暂无好友</span><small>输入用户名添加好友。</small></li>';
    return;
  }
  state.friends.forEach((friend) => {
    const li = document.createElement('li');
    li.innerHTML = `
      <span class="friend-line">${avatarMarkup(friend.avatar)} ${escapeHtml(friend.name)}</span>
      <div class="row-actions">
        <button class="secondary-btn" type="button" data-action="chat">发消息</button>
        <button class="secondary-btn danger-text" type="button" data-action="remove">删除</button>
      </div>
    `;
    li.querySelector('[data-action="chat"]').addEventListener('click', () => {
      DOM.contactsDialog.close();
      selectPeer(friend.name, friend.avatar);
    });
    li.querySelector('[data-action="remove"]').addEventListener('click', () => removeFriend(friend.name));
    DOM.friendsList.appendChild(li);
  });
}

async function loadFriendsDialog() {
  DOM.friendsList.innerHTML = '<li><span>加载中...</span></li>';
  DOM.contactsDialog.showModal();
  await refreshFriends();
  renderFriendsDialog();
}

async function addFriend(friendName = DOM.friendNameInput.value.trim() || state.selectedPeer) {
  if (!friendName) return toast('请输入好友用户名', 'error');
  if (friendName === state.username) return toast('不能添加自己为好友', 'error');
  await api('/api/friend', {
    method: 'POST',
    body: { action: 'add', friend: friendName }
  });
  DOM.friendNameInput.value = '';
  await refreshFriends();
  renderFriendsDialog();
  toast('好友已添加');
}

async function removeFriend(friendName) {
  await api('/api/friend', {
    method: 'POST',
    body: { action: 'remove', friend: friendName }
  });
  await refreshFriends();
  renderFriendsDialog();
  toast('好友已删除');
}
function bindEvents() {
  DOM.loginForm.addEventListener('submit', (event) => event.preventDefault());
  DOM.connectBtn.addEventListener('click', () => login().catch((err) => toast(err.message, 'error')));
  DOM.registerBtn.addEventListener('click', () => register().catch((err) => toast(err.message, 'error')));
  DOM.sendBtn.addEventListener('click', () => sendMessage().catch((err) => toast(err.message, 'error')));
  DOM.messageInput.addEventListener('keydown', (event) => {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      sendMessage().catch((err) => toast(err.message, 'error'));
    }
  });

  document.querySelectorAll('[data-mode]').forEach((btn) => btn.addEventListener('click', () => setMode(btn.dataset.mode)));
  document.querySelectorAll('[data-mode-jump]').forEach((btn) => btn.addEventListener('click', () => setMode(btn.dataset.modeJump)));
  document.querySelectorAll('.tab').forEach((btn) => btn.addEventListener('click', () => {
    document.querySelectorAll('.tab').forEach((item) => item.classList.remove('active'));
    btn.classList.add('active');
    applySearchFilter(btn.dataset.filter);
  }));

  DOM.globalSearchInput.addEventListener('input', () => applySearchFilter());
  DOM.toUser.addEventListener('input', updateChatTitle);
  DOM.toGroup.addEventListener('input', () => {
    DOM.inviteGroupName.value = DOM.toGroup.value.trim();
    updateChatTitle();
  });
  DOM.loadHistoryBtn.addEventListener('click', () => loadHistory().catch((err) => toast(err.message, 'error')));
  DOM.logoutBtn.addEventListener('click', () => logout());
  DOM.refreshOnlineBtn.addEventListener('click', () => Promise.allSettled([refreshOnline(), refreshFriends()]).then(() => toast('在线列表已刷新')));
  DOM.openAddFriendBtn.addEventListener('click', () => {
    DOM.contactsDialog.showModal();
    DOM.friendNameInput.focus();
  });
  document.querySelector('.rail-logo')?.addEventListener('click', () => {
    setMode('public');
    loadHistory('public').catch((err) => toast(err.message, 'error'));
  });
  DOM.newMessageBtn.addEventListener('click', () => {
    setMode('public');
    DOM.messageInput.focus();
    loadHistory('public').catch((err) => toast(err.message, 'error'));
  });

  $('focusSearchBtn').addEventListener('click', () => DOM.globalSearchInput.focus());
  $('callBtn').addEventListener('click', () => toast('语音通话入口已响应'));
  $('videoBtn').addEventListener('click', () => toast('视频通话入口已响应'));
  $('clearViewBtn').addEventListener('click', () => resetMessages('已清空当前视图', '这只影响前端显示，不会删除服务器历史。'));
  $('moreBtn').addEventListener('click', () => addFriend().catch((err) => toast(err.message, 'error')));
  $('emojiBtn').addEventListener('click', (event) => {
    event.stopPropagation();
    const picker = $('emojiPicker');
    picker.hidden = !picker.hidden;
  });
  $('emojiPicker').addEventListener('click', (event) => {
    const btn = event.target.closest('button');
    if (!btn) return;
    const emoji = btn.textContent;
    const input = DOM.messageInput;
    const start = input.selectionStart;
    const end = input.selectionEnd;
    input.value = input.value.slice(0, start) + emoji + input.value.slice(end);
    input.selectionStart = input.selectionEnd = start + emoji.length;
    input.focus();
    $('emojiPicker').hidden = true;
  });
  document.addEventListener('click', (event) => {
    const picker = $('emojiPicker');
    const btn = $('emojiBtn');
    if (!picker.hidden && !picker.contains(event.target) && event.target !== btn && !btn.contains(event.target)) {
      picker.hidden = true;
    }
  });

  DOM.headerAvatarBtn.addEventListener('click', () => openProfile(state.username));
  DOM.profileAvatarBtn.addEventListener('click', () => openProfile(state.username));
  DOM.viewProfileBtn.addEventListener('click', () => openProfile(state.username));
  DOM.chatAvatarBtn.addEventListener('click', () => {
    if (state.mode === 'private' && state.selectedPeer) {
      openProfile(state.selectedPeer);
    } else {
      openProfile(state.username);
    }
  });
  DOM.saveProfileBtn.addEventListener('click', () => saveProfile().catch((err) => toast(err.message, 'error')));
  DOM.saveAvatarBtn.addEventListener('click', () => saveAvatar().catch((err) => toast(err.message, 'error')));
  DOM.createGroupBtn.addEventListener('click', () => createOrJoinGroup('create').catch((err) => toast(err.message, 'error')));
  DOM.joinGroupBtn.addEventListener('click', () => createOrJoinGroup('join').catch((err) => toast(err.message, 'error')));
  DOM.inviteGroupBtn.addEventListener('click', () => inviteToGroup().catch((err) => toast(err.message, 'error')));
  DOM.addFriendBtn.addEventListener('click', () => addFriend(DOM.friendNameInput.value.trim()).catch((err) => toast(err.message, 'error')));
  DOM.reloadFriendsBtn.addEventListener('click', () => loadFriendsDialog().catch((err) => toast(err.message, 'error')));

  ['groupsRailBtn', 'groupsTopBtn'].forEach((id) => $(id)?.addEventListener('click', () => refreshGroups().then(() => DOM.groupsDialog.showModal()).catch((err) => toast(err.message, 'error'))));
  ['contactsRailBtn', 'contactsTopBtn'].forEach((id) => $(id)?.addEventListener('click', () => loadFriendsDialog().catch((err) => toast(err.message, 'error'))));
  ['settingsTopBtn', 'settingsRailBtn'].forEach((id) => $(id)?.addEventListener('click', () => DOM.settingsDialog.showModal()));
  DOM.quickAddFriendBtn?.addEventListener('click', () => {
    DOM.contactsDialog.showModal();
    DOM.friendNameInput.focus();
  });
  DOM.quickCreateGroupBtn?.addEventListener('click', () => {
    DOM.groupsDialog.showModal();
    DOM.newGroupName.focus();
  });
  DOM.quickUploadAvatarBtn?.addEventListener('click', () => openProfile(state.username));
  DOM.themeQuickBtn?.addEventListener('click', () => {
    DOM.darkModeToggle.checked = !DOM.darkModeToggle.checked;
    DOM.darkModeToggle.dispatchEvent(new Event('change'));
  });
  DOM.imageToolBtn?.addEventListener('click', () => toast('图片发送入口已准备，当前后端未提供文件消息接口'));
  DOM.fileToolBtn?.addEventListener('click', () => toast('文件发送入口已准备，当前后端未提供文件消息接口'));
  $('discoverRailBtn').addEventListener('click', () => openDiscover().catch((err) => toast(err.message, 'error')));
  $('discoverTopBtn').addEventListener('click', () => openDiscover().catch((err) => toast(err.message, 'error')));
  DOM.discoverRefreshBtn.addEventListener('click', () => openDiscover().catch((err) => toast(err.message, 'error')));
  DOM.publishThoughtBtn.addEventListener('click', () => publishThought());
  DOM.topicTags.addEventListener('click', (event) => {
    const btn = event.target.closest('.topic-tag');
    if (btn) btn.classList.toggle('active');
  });
  DOM.thoughtInput.addEventListener('keydown', (event) => {
    if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
      event.preventDefault();
      publishThought();
    }
  });
  DOM.thoughtFeed.addEventListener('click', (event) => {
    const button = event.target.closest('button[data-action]');
    if (!button) return;
    const card = button.closest('.thought-card');
    const thought = state.thoughts.find((item) => item.id === card?.dataset.id);
    if (!thought) return;
    const action = button.dataset.action;
    if (action === 'profile') {
      openProfile(thought.author);
      return;
    }
    if (action === 'chat') {
      DOM.discoverDialog.close();
      const friend = state.friends.find((item) => item.name === thought.author);
      selectPeer(thought.author, friend?.avatar || thought.avatar);
      return;
    }
    if (action === 'like') {
      updateThought(thought.id, (item) => {
        item.likedBy = Array.isArray(item.likedBy) ? item.likedBy : [];
        if (item.likedBy.includes(state.username)) {
          item.likedBy = item.likedBy.filter((name) => name !== state.username);
        } else {
          item.likedBy.push(state.username);
        }
        item.likes = item.likedBy.length;
      });
      return;
    }
    if (action === 'comment') {
      const input = card.querySelector('[data-role="comment-input"]');
      const content = input.value.trim();
      if (!content) return toast('评论不能为空', 'error');
      updateThought(thought.id, (item) => {
        item.comments = Array.isArray(item.comments) ? item.comments : [];
        item.comments.push({ author: state.username, content, createdAt: new Date().toISOString() });
      });
    }
  });
  DOM.discoverAddFriendBtn.addEventListener('click', () => {
    DOM.discoverDialog.close();
    DOM.contactsDialog.showModal();
    DOM.friendNameInput.focus();
  });
  DOM.discoverPublicBtn.addEventListener('click', () => {
    DOM.discoverDialog.close();
    setMode('public');
    loadHistory('public').catch((err) => toast(err.message, 'error'));
  });
  DOM.discoverOnlineBtn.addEventListener('click', () => {
    DOM.discoverDialog.close();
    DOM.globalSearchInput.focus();
  });
  DOM.discoverFriendsBtn.addEventListener('click', () => {
    DOM.discoverDialog.close();
    loadFriendsDialog().catch((err) => toast(err.message, 'error'));
  });
  DOM.discoverGroupsBtn.addEventListener('click', () => {
    DOM.discoverDialog.close();
    refreshGroups().then(() => DOM.groupsDialog.showModal()).catch((err) => toast(err.message, 'error'));
  });

  DOM.darkModeToggle.addEventListener('change', () => {
    document.body.classList.toggle('dark-mode', DOM.darkModeToggle.checked);
    localStorage.setItem('darkMode', String(DOM.darkModeToggle.checked));
  });
  DOM.soundToggle.addEventListener('change', () => localStorage.setItem('soundEnabled', String(DOM.soundToggle.checked)));
  DOM.fontSizeSlider.addEventListener('input', () => {
    DOM.fontSizeValue.textContent = `${DOM.fontSizeSlider.value}px`;
    document.body.style.fontSize = `${DOM.fontSizeSlider.value}px`;
    localStorage.setItem('fontSize', DOM.fontSizeSlider.value);
  });
  document.addEventListener('keydown', (event) => {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
      event.preventDefault();
      DOM.globalSearchInput.focus();
      DOM.globalSearchInput.select();
    }
  });

  bindAvatarInput(DOM.loginAvatarUpload, DOM.loginAvatarPreview);
  bindAvatarInput(DOM.settingsAvatarUpload, DOM.settingsAvatarPreview);
}

function init() {
  DOM.darkModeToggle.checked = localStorage.getItem('darkMode') === 'true';
  DOM.soundToggle.checked = localStorage.getItem('soundEnabled') !== 'false';
  DOM.fontSizeSlider.value = localStorage.getItem('fontSize') || '14';
  DOM.fontSizeValue.textContent = `${DOM.fontSizeSlider.value}px`;
  document.body.style.fontSize = `${DOM.fontSizeSlider.value}px`;
  document.body.classList.toggle('dark-mode', DOM.darkModeToggle.checked);
  loadThoughts();
  renderSessionList();
  renderGroupSessions();
  renderSidePanels();
  setConnectionStatus(false);
  resetMessages();
  bindEvents();
}

init();
