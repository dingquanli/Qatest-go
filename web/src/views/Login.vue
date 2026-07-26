<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Zap, Eye, EyeOff, Shield, AlertCircle } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import { useUserStore } from '@/stores/user'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const username = ref('')
const password = ref('')
const showPwd = ref(false)
const error = ref('')
const loading = ref(false)

const redirect = computed(() => (route.query.redirect as string) || '/dashboard')

async function handleLogin() {
  if (!username.value.trim() || !password.value.trim()) {
    error.value = '请输入账号和密码'
    return
  }
  loading.value = true
  error.value = ''
  try {
    await userStore.login({ username: username.value, password: password.value })
    router.push(redirect.value)
  } catch (e) {
    error.value = e instanceof Error ? e.message : '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-gradient-to-br from-background via-background to-primary/5">
    <div class="w-full max-w-sm">
      <!-- Logo -->
      <div class="text-center mb-8">
        <div class="inline-flex items-center justify-center w-14 h-14 rounded-2xl bg-primary text-primary-foreground mb-4">
          <Zap class="w-7 h-7" />
        </div>
        <h1 class="text-2xl font-bold tracking-tight">Qatest</h1>
        <p class="text-sm text-muted-foreground mt-1">全流程测试平台</p>
      </div>

      <!-- 登录表单 -->
      <div class="bg-card border rounded-2xl shadow-lg p-6 space-y-4">
        <div>
          <label class="text-sm font-medium mb-1.5 block">账号</label>
          <Input
            v-model="username"
            placeholder="请输入账号"
            class="h-10"
            @keydown.enter="handleLogin"
          />
        </div>
        <div>
          <label class="text-sm font-medium mb-1.5 block">密码</label>
          <div class="relative">
            <Input
              :type="showPwd ? 'text' : 'password'"
              v-model="password"
              placeholder="请输入密码"
              class="h-10 pr-10"
              @keydown.enter="handleLogin"
            />
            <button
              type="button"
              @click="showPwd = !showPwd"
              class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
            >
              <EyeOff v-if="showPwd" class="w-4 h-4" />
              <Eye v-else class="w-4 h-4" />
            </button>
          </div>
        </div>

        <div v-if="error" class="p-3 rounded-lg bg-destructive/10 border border-destructive/20 flex items-center gap-2">
          <AlertCircle class="w-4 h-4 text-destructive shrink-0" />
          <p class="text-sm text-destructive">{{ error }}</p>
        </div>

        <Button class="w-full h-10" @click="handleLogin" :disabled="loading">
          {{ loading ? '登录中...' : '登录' }}
        </Button>
      </div>

      <!-- 安全提示 -->
      <div class="mt-6 p-4 rounded-xl bg-muted/50 border border-border/50">
        <div class="flex items-start gap-3">
          <Shield class="w-5 h-5 text-primary shrink-0 mt-0.5" />
          <div class="text-sm text-muted-foreground">
            <p class="font-medium text-foreground mb-1">安全提示</p>
            <p>请使用您的账号密码登录</p>
            <p class="mt-1">生产环境请配置HTTPS以保证传输安全</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
