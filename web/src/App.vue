<template>
  <div id="app">
    <header class="navbar">
      <h1>VisionLoop</h1>
      <nav>
        <router-link to="/">实时监看</router-link>
        <router-link to="/playback">历史回放</router-link>
        <router-link to="/events">事件中心</router-link>
        <router-link to="/settings">设置</router-link>
      </nav>
    </header>
    <main class="container">
      <router-view />
    </main>
  </div>
</template>

<script setup>
</script>

<style>
#app {
  min-height: 100vh;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  background: #0f0f0f;
  color: #fff;
}
.navbar {
  background: #1a1a1a;
  padding: 1rem 2rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid #333;
}
.navbar h1 {
  font-size: 1.5rem;
  color: #00d4ff;
}
.navbar nav {
  display: flex;
  gap: 1.5rem;
}
.navbar nav a {
  color: #aaa;
  text-decoration: none;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  transition: all 0.2s;
}
.navbar nav a:hover,
.navbar nav a.active {
  color: #fff;
  background: #00d4ff22;
}
.container {
  max-width: 1400px;
  margin: 0 auto;
  padding: 2rem;
}
</style>
