import { useState } from 'react'
import type { FormEvent } from 'react'

import { Button, Field, Input } from '@/components/ui'
import { ApiError, changeSaaSPassword, completeSaaSMFA, loginSaaS, persistSaaSLogin } from '@/lib/api'

type LoginStage = 'credentials' | 'mfa' | 'password'

export default function LoginPage() {
  const [stage, setStage] = useState<LoginStage>('credentials')
  const [login, setLogin] = useState('')
  const [password, setPassword] = useState('')
  const [code, setCode] = useState('')
  const [challengeToken, setChallengeToken] = useState('')
  const [enrollmentSecret, setEnrollmentSecret] = useState('')
  const [otpAuthURL, setOtpAuthURL] = useState('')
  const [passwordChangeToken, setPasswordChangeToken] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const finish = (result: Awaited<ReturnType<typeof loginSaaS>>) => {
    if (!persistSaaSLogin(result)) {
      setError('认证响应不完整，请重新登录')
      return
    }
    location.assign('/saas-admin/')
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setLoading(true)
    setError('')
    try {
      if (stage === 'credentials') {
        const result = await loginSaaS(login, password)
        if (result.enrollmentToken || result.challengeToken) {
          setChallengeToken(result.enrollmentToken || result.challengeToken || '')
          setEnrollmentSecret(result.enrollmentSecret || '')
          setOtpAuthURL(result.otpAuthURL || '')
          setStage('mfa')
        } else if (result.passwordChangeToken) {
          setPasswordChangeToken(result.passwordChangeToken)
          setStage('password')
        } else {
          finish(result)
        }
      } else if (stage === 'mfa') {
        const result = await completeSaaSMFA(challengeToken, code)
        if (result.passwordChangeToken) {
          setPasswordChangeToken(result.passwordChangeToken)
          setStage('password')
        } else {
          finish(result)
        }
      } else {
        finish(await changeSaaSPassword(passwordChangeToken, newPassword))
      }
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : '认证失败，请稍后重试')
    } finally {
      setLoading(false)
    }
  }

  const title = stage === 'credentials' ? 'SaaS 管理员登录' : stage === 'mfa' ? '完成多因素认证' : '首次登录设置密码'
  const description = stage === 'credentials'
    ? '使用平台管理员身份进入 SaaS 总后台。'
    : stage === 'mfa'
      ? '请输入认证器生成的一次性验证码。'
      : '初始化管理员必须先设置新的登录密码。'

  return (
    <main className="flex min-h-screen items-center justify-center bg-[#f4f5f7] px-4 py-10">
      <section className="w-full max-w-md rounded-xl border border-zinc-200 bg-white p-6 shadow-sm sm:p-8">
        <div className="mb-7">
          <div className="mb-4 flex h-10 w-10 items-center justify-center rounded-lg bg-emerald-700 text-lg font-semibold text-white">M</div>
          <h1 className="text-2xl font-semibold text-zinc-950">{title}</h1>
          <p className="mt-2 text-sm text-zinc-500">{description}</p>
        </div>
        <form className="grid gap-4" onSubmit={submit}>
          {stage === 'credentials' && <>
            <Field label="登录名">
              <Input autoComplete="username" value={login} onChange={(event) => setLogin(event.target.value)} required />
            </Field>
            <Field label="密码">
              <Input type="password" autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} required />
            </Field>
          </>}
          {stage === 'mfa' && <Field label="验证码" hint="挑战过期后请返回重新登录。">
            {enrollmentSecret && <p className="mb-2 rounded-md bg-amber-50 px-3 py-2 text-xs text-amber-900">首次设置请保存此一次性密钥：{enrollmentSecret}{otpAuthURL ? `（otpauth 地址：${otpAuthURL}）` : ''}</p>}
            <Input inputMode="numeric" autoComplete="one-time-code" value={code} onChange={(event) => setCode(event.target.value)} required />
          </Field>}
          {stage === 'password' && <Field label="新密码" hint="请勿使用初始化密码。">
            <Input type="password" autoComplete="new-password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} required />
          </Field>}
          {error && <p role="alert" className="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>}
          <Button type="submit" loading={loading} className="mt-2 h-10 w-full">
            {stage === 'credentials' ? '登录' : stage === 'mfa' ? '验证并继续' : '保存新密码'}
          </Button>
        </form>
      </section>
    </main>
  )
}
