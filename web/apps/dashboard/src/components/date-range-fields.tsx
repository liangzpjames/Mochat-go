import { Button } from 'antd';
import { useState } from 'react';

export type DateRangeValue = { startDate: string; endDate: string };

export function validateDateRange(value: DateRangeValue): string | null {
  return value.startDate && value.endDate && value.startDate > value.endDate
    ? '开始日期不能晚于结束日期'
    : null;
}

export function DateRangeFields({
  value,
  onChange,
  onValidSubmit,
  startLabel = '开始日期',
  endLabel = '结束日期',
  submitLabel = '查询',
}: {
  value: DateRangeValue;
  onChange: (value: DateRangeValue) => void;
  onValidSubmit?: (value: DateRangeValue) => void;
  startLabel?: string;
  endLabel?: string;
  submitLabel?: string;
}) {
  const [error, setError] = useState<string | null>(() => validateDateRange(value));

  function update(next: DateRangeValue) {
    setError(validateDateRange(next));
    onChange(next);
  }

  function submit() {
    const nextError = validateDateRange(value);
    setError(nextError);
    if (nextError === null) onValidSubmit?.(value);
  }

  return (
    <div className="date-range-fields">
      <label>
        <span>{startLabel}</span>
        <input
          aria-invalid={error !== null}
          type="date"
          value={value.startDate}
          onChange={(event) => update({ ...value, startDate: event.target.value })}
        />
      </label>
      <label>
        <span>{endLabel}</span>
        <input
          aria-invalid={error !== null}
          type="date"
          value={value.endDate}
          onChange={(event) => update({ ...value, endDate: event.target.value })}
        />
      </label>
      {onValidSubmit !== undefined && <Button aria-label={submitLabel} type="primary" onClick={submit}>{submitLabel}</Button>}
      {error !== null && <p className="date-range-fields__error" role="alert">{error}</p>}
    </div>
  );
}
