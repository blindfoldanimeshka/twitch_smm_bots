import { useState, useCallback, useEffect } from 'react'

export interface BotFormData {
  id: string
  twitch_login: string
  password: string
  name: string
  personality: string
  min_delay_sec: number
  max_delay_sec: number
  rate_limit_per_min: number
}

export interface ValidationErrors {
  id?: string
  twitch_login?: string
  password?: string
  name?: string
  min_delay_sec?: string
  max_delay_sec?: string
  rate_limit_per_min?: string
}

export interface TouchedFields {
  [key: string]: boolean
}

const buildFormData = (editingBot?: any): BotFormData => {
  if (!editingBot) {
    return initialFormData
  }

  return {
    id: editingBot.id,
    twitch_login: editingBot.twitch_login,
    password: '',
    name: editingBot.name,
    personality: editingBot.personality,
    min_delay_sec: editingBot.min_delay_sec,
    max_delay_sec: editingBot.max_delay_sec,
    rate_limit_per_min: editingBot.rate_limit_per_min,
  }
}

const initialFormData: BotFormData = {
  id: '',
  twitch_login: '',
  password: '',
  name: '',
  personality: 'нейтрал',
  min_delay_sec: 45,
  max_delay_sec: 180,
  rate_limit_per_min: 10,
}

export function useBotForm(editingBot?: any) {
  const [formData, setFormData] = useState<BotFormData>(buildFormData(editingBot))
  const [touchedFields, setTouchedFields] = useState<TouchedFields>({})
  const [showAdvanced, setShowAdvanced] = useState(false)

  useEffect(() => {
    setFormData(buildFormData(editingBot))
    setTouchedFields({})
  }, [editingBot])

  const validateField = useCallback(
    (name: keyof BotFormData, value: any): string | undefined => {
      switch (name) {
        case 'id':
          if (!value || value.trim() === '') return 'ID обязателен'
          if (value.length < 3) return 'ID должен быть минимум 3 символа'
          if (!/^[a-zA-Z0-9_-]+$/.test(value))
            return 'ID может содержать только буквы, цифры, _ и -'
          break

        case 'twitch_login':
          if (!value || value.trim() === '') return 'Twitch логин обязателен'
          if (value.length < 4 || value.length > 25)
            return 'Twitch логин должен быть 4-25 символов'
          if (!/^[a-zA-Z0-9_]+$/.test(value))
            return 'Twitch логин может содержать только буквы, цифры и _'
          break

        case 'password':
          if (!editingBot && (!value || value.trim() === ''))
            return 'Пароль обязателен'
          if (value && value.length < 3)
            return 'Пароль должен быть минимум 3 символа'
          break

        case 'name':
          if (!value || value.trim() === '') return 'Имя обязательно'
          if (value.length < 2) return 'Имя должно быть минимум 2 символа'
          break

        case 'min_delay_sec':
          if (value < 10) return 'Минимальная задержка: 10 сек'
          if (value > 300) return 'Максимальная задержка: 300 сек'
          if (value >= formData.max_delay_sec)
            return 'Должна быть меньше максимальной'
          break

        case 'max_delay_sec':
          if (value < 20) return 'Минимальная задержка: 20 сек'
          if (value > 600) return 'Максимальная задержка: 600 сек'
          if (value <= formData.min_delay_sec)
            return 'Должна быть больше минимальной'
          break

        case 'rate_limit_per_min':
          if (value < 1) return 'Минимум: 1 сообщение/мин'
          if (value > 20) return 'Рекомендуется не более 20 для безопасности'
          break
      }
      return undefined
    },
    [formData.min_delay_sec, formData.max_delay_sec, editingBot]
  )

  const errors: ValidationErrors = {}
  Object.keys(formData).forEach((key) => {
    const error = validateField(key as keyof BotFormData, formData[key as keyof BotFormData])
    if (error) {
      errors[key as keyof ValidationErrors] = error
    }
  })

  const isValid = Object.keys(errors).length === 0

  const handleChange = useCallback(
    (name: keyof BotFormData, value: any) => {
      setFormData((prev) => ({ ...prev, [name]: value }))
    },
    []
  )

  const handleBlur = useCallback((name: keyof BotFormData) => {
    setTouchedFields((prev) => ({ ...prev, [name]: true }))
  }, [])

  const reset = useCallback(() => {
    setFormData(buildFormData(editingBot))
    setTouchedFields({})
    setShowAdvanced(false)
  }, [editingBot])

  return {
    formData,
    errors,
    touchedFields,
    isValid,
    showAdvanced,
    setShowAdvanced,
    handleChange,
    handleBlur,
    reset,
  }
}
