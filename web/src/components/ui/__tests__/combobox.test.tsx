/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'

import { Dialog } from '@/components/dialog'

import { Combobox } from '../combobox'
import { ComboboxInput } from '../combobox-input'
import { Sheet, SheetContent, SheetTitle } from '../sheet'

const options = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Google' },
  { value: 'disabled', label: 'Unavailable provider', disabled: true },
]

function Fixture() {
  const [value, setValue] = useState('openai')
  return (
    <>
      <Combobox
        options={options}
        value={value}
        onValueChange={(next) => setValue(next ?? '')}
        aria-label='Provider'
        emptyText='No matching provider'
      />
      <output>{value}</output>
    </>
  )
}

describe('searchable single selection', () => {
  it.each(['dialog', 'sheet'] as const)(
    'keeps the options closed when a %s autofocuses the input, then opens on click',
    async (container) => {
      const field = <Fixture />
      render(
        container === 'dialog' ? (
          <Dialog open title='Choose a provider'>
            {field}
          </Dialog>
        ) : (
          <Sheet open>
            <SheetContent>
              <SheetTitle>Choose a provider</SheetTitle>
              {field}
            </SheetContent>
          </Sheet>
        )
      )
      const user = userEvent.setup()
      const input = screen.getByRole('combobox', { name: 'Provider' })
      await waitFor(() => expect(input).toHaveFocus())
      expect(input).toHaveAttribute('aria-expanded', 'false')
      expect(input).toHaveValue('OpenAI')
      expect(screen.queryByRole('listbox')).not.toBeInTheDocument()

      await user.click(input)
      expect(screen.getByRole('option', { name: 'Google' })).toBeVisible()
      await user.click(screen.getByRole('option', { name: 'Google' }))
      await waitFor(() => expect(input).toHaveValue('Google'))
      expect(input).toHaveAttribute('aria-expanded', 'false')
      expect(screen.getByRole('dialog')).toBeVisible()
    }
  )

  it('keeps options closed on Tab focus and opens them with the arrow key', async () => {
    render(<Fixture />)
    const user = userEvent.setup()
    const input = screen.getByRole('combobox', { name: 'Provider' })
    await user.tab()
    expect(input).toHaveFocus()
    expect(input).toHaveAttribute('aria-expanded', 'false')
    await user.keyboard('{ArrowDown}')
    expect(screen.getByRole('option', { name: 'Google' })).toBeVisible()
    await user.keyboard('{Escape}')
    expect(input).toHaveAttribute('aria-expanded', 'false')
    expect(input).toHaveValue('OpenAI')
  })

  it('opens and filters when typing into a focused, closed input', async () => {
    render(<Fixture />)
    const user = userEvent.setup()
    const input = screen.getByRole('combobox', { name: 'Provider' })
    await user.tab()
    await user.keyboard('g')
    expect(input).toHaveValue('g')
    expect(screen.getByRole('option', { name: 'Google' })).toBeVisible()
    expect(
      screen.queryByRole('option', { name: 'OpenAI' })
    ).not.toBeInTheDocument()
  })

  it.each([true, false])(
    'honors explicit openOnFocus=%s',
    async (openOnFocus) => {
      render(
        <Combobox
          options={options}
          value='openai'
          aria-label='Provider'
          openOnFocus={openOnFocus}
        />
      )
      const user = userEvent.setup()
      const input = screen.getByRole('combobox', { name: 'Provider' })
      await user.tab()
      expect(input).toHaveFocus()
      expect(input).toHaveAttribute('aria-expanded', String(openOnFocus))
      if (!openOnFocus) {
        await user.click(screen.getByRole('button', { name: 'Provider' }))
        expect(screen.getByRole('option', { name: 'Google' })).toBeVisible()
      }
    }
  )

  it('keeps openOnFocus=false after clicking an input that already had focus', async () => {
    render(
      <>
        <button type='button'>Before</button>
        <ComboboxInput
          options={options}
          value='openai'
          onValueChange={() => {}}
          aria-label='Provider'
          openOnFocus={false}
        />
      </>
    )
    const user = userEvent.setup()
    const input = screen.getByRole('combobox', { name: 'Provider' })
    await user.tab()
    await user.tab()
    expect(input).toHaveFocus()
    await user.click(input)
    expect(input).toHaveAttribute('aria-expanded', 'true')
    await user.keyboard('{Escape}')
    await user.tab({ shift: true })
    await user.tab()
    expect(input).toHaveFocus()
    expect(input).toHaveAttribute('aria-expanded', 'false')
    expect(input).toHaveValue('OpenAI')
  })

  it('searches labels and values without committing text, shows empty results, and restores the selection on Escape', async () => {
    render(<Fixture />)
    const user = userEvent.setup()
    const input = screen.getByRole('combobox', { name: 'Provider' })
    expect(input).toHaveValue('OpenAI')
    await user.click(input)
    await user.type(input, 'missing')
    expect(screen.getByText('No matching provider')).toBeVisible()
    expect(screen.getByText('openai')).toHaveTextContent('openai')
    await user.keyboard('{Escape}')
    expect(input).toHaveValue('OpenAI')
    await user.click(input)
    await user.type(input, 'gemini')
    expect(screen.getByRole('option', { name: 'Google' })).toBeVisible()
    await user.keyboard('{ArrowDown}{Enter}')
    await waitFor(() => expect(input).toHaveValue('Google'))
    expect(screen.getByText('gemini')).toHaveTextContent('gemini')
  })

  it('respects disabled controls and options', async () => {
    const change = vi.fn()
    const view = render(
      <Combobox
        options={options}
        value='openai'
        onValueChange={change}
        aria-label='Provider'
        disabled
      />
    )
    const user = userEvent.setup()
    expect(screen.getByRole('combobox', { name: 'Provider' })).toBeDisabled()
    view.rerender(
      <Combobox
        options={options}
        value='openai'
        onValueChange={change}
        aria-label='Provider'
      />
    )
    await user.click(screen.getByRole('combobox', { name: 'Provider' }))
    expect(
      screen.getByRole('option', { name: 'Unavailable provider' })
    ).toHaveAttribute('aria-disabled', 'true')
    await user.click(
      screen.getByRole('option', { name: 'Unavailable provider' })
    )
    expect(change).not.toHaveBeenCalled()
  })
})

const pluginOptions = [
  {
    value: 'alpha',
    label: 'Alpha plugin',
    icon: <img src='/api/plugin/task/alpha/icon' alt='' />,
  },
  {
    value: 'beta',
    label: 'Beta plugin',
    icon: <img src='/api/plugin/task/beta/icon' alt='' />,
  },
]

function PluginSelectionFixture() {
  const [value, setValue] = useState<string | null>('alpha')
  return (
    <Combobox
      options={pluginOptions}
      value={value}
      onValueChange={setValue}
      showSelectedIcon
      aria-label='Task plugin'
    />
  )
}

describe('selected option icons', () => {
  it('shows the selected plugin logo and updates it when choosing another plugin', async () => {
    render(<PluginSelectionFixture />)
    const user = userEvent.setup()
    const input = screen.getByRole('combobox', { name: 'Task plugin' })
    expect(screen.getByAltText('')).toHaveAttribute(
      'src',
      '/api/plugin/task/alpha/icon'
    )
    await user.click(input)
    const nextOption = screen.getByRole('option', { name: 'Beta plugin' })
    expect(nextOption.querySelector('img')).toHaveAttribute(
      'src',
      '/api/plugin/task/beta/icon'
    )
    await user.click(nextOption)
    await waitFor(() => expect(input).toHaveValue('Beta plugin'))
    expect(screen.getByAltText('')).toHaveAttribute(
      'src',
      '/api/plugin/task/beta/icon'
    )
  })

  it('removes the logo when the selection is cleared or no longer has an icon', () => {
    const view = render(
      <Combobox
        options={pluginOptions}
        value='alpha'
        showSelectedIcon
        aria-label='Task plugin'
      />
    )
    expect(screen.getByAltText('')).toBeInTheDocument()
    view.rerender(
      <Combobox
        options={pluginOptions}
        value={null}
        showSelectedIcon
        aria-label='Task plugin'
      />
    )
    expect(screen.queryByAltText('')).not.toBeInTheDocument()
    view.rerender(
      <Combobox
        options={[{ value: 'alpha', label: 'Alpha plugin' }]}
        value='alpha'
        showSelectedIcon
        aria-label='Task plugin'
      />
    )
    expect(screen.queryByAltText('')).not.toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Task plugin' })).toHaveValue(
      'Alpha plugin'
    )
  })

  it('preserves the existing text-only selected state unless icon display is requested', () => {
    render(
      <Combobox
        options={pluginOptions}
        value='alpha'
        aria-label='Task plugin'
      />
    )
    expect(screen.queryByAltText('')).not.toBeInTheDocument()
  })
})

describe('ComboboxInput dropdown placement', () => {
  const inputRect = {
    left: 340,
    top: 180,
    right: 740,
    bottom: 212,
    width: 400,
    height: 32,
  }
  const centered = 'translate(-50%, -50%)'

  function rect(box: Partial<DOMRect> & { left: number; top: number }) {
    return () =>
      ({ x: box.left, y: box.top, ...box, toJSON: () => ({}) }) as DOMRect
  }

  // jsdom has no layout: an element whose inline transform is set stands for a
  // fixed-position containing block, anything else leaves it to the viewport.
  function containingBlock(el: HTMLElement) {
    for (let node = el.parentElement; node; node = node.parentElement) {
      const transform = node.style.transform
      if (transform && transform !== 'none') return node
    }
    return null
  }

  function visualPosition(listbox: HTMLElement) {
    const wrapper = listbox.parentElement as HTMLElement
    const left = parseFloat(wrapper.style.left)
    const top = parseFloat(wrapper.style.top)
    const width = parseFloat(wrapper.style.width)
    const block = containingBlock(wrapper)
    if (!block) return { left, top, width }
    const box = block.getBoundingClientRect()
    const scale =
      box.width && block.offsetWidth ? box.width / block.offsetWidth : 1
    return {
      left: box.left + block.clientLeft + (left - block.scrollLeft) * scale,
      top: box.top + block.clientTop + (top - block.scrollTop) * scale,
      width: width * scale,
    }
  }

  function mockLayout(
    input: HTMLElement,
    dialog: HTMLElement | null,
    options: {
      scroll?: { top: number; left: number }
      dialogBox?: { left: number; top: number }
      transform?: string
    } = {}
  ) {
    input.getBoundingClientRect = rect(inputRect)
    if (!dialog) return
    const {
      scroll = { top: 0, left: 0 },
      dialogBox = { left: 300, top: 100 },
    } = options
    if (options.transform) dialog.style.transform = options.transform
    dialog.getBoundingClientRect = rect(dialogBox)
    Object.defineProperty(dialog, 'scrollTop', {
      value: scroll.top,
      configurable: true,
    })
    Object.defineProperty(dialog, 'scrollLeft', {
      value: scroll.left,
      configurable: true,
    })
  }

  function field() {
    return (
      <ComboboxInput
        options={options}
        value='openai'
        onValueChange={() => {}}
        aria-label='Provider'
      />
    )
  }

  const underInput = { left: 340, top: 216, width: 400 }

  it.each([
    { scrollTop: 0, scrollLeft: 0 },
    { scrollTop: 50, scrollLeft: 0 },
  ])(
    'sits right under the input inside a transformed dialog scrolled by %o',
    async (scroll) => {
      render(
        <Dialog open title='Choose a provider'>
          {field()}
        </Dialog>
      )
      const dialog = screen.getByRole('dialog')
      const input = screen.getByRole('combobox', { name: 'Provider' })
      mockLayout(input, dialog, {
        transform: centered,
        scroll: { top: scroll.scrollTop, left: scroll.scrollLeft },
      })
      await waitFor(() => expect(input).toHaveFocus())
      await userEvent.setup().click(input)
      const listbox = await screen.findByRole('listbox')
      expect(visualPosition(listbox)).toEqual(underInput)
    }
  )

  it('sits right under the input inside a sheet that has no transform', async () => {
    render(
      <Sheet open>
        <SheetContent>
          <SheetTitle>Choose a provider</SheetTitle>
          {field()}
        </SheetContent>
      </Sheet>
    )
    const sheet = screen.getByRole('dialog')
    const input = screen.getByRole('combobox', { name: 'Provider' })
    mockLayout(input, sheet, { dialogBox: { left: 1000, top: 0 } })
    input.getBoundingClientRect = rect({
      ...inputRect,
      left: 1050,
      right: 1450,
    })
    await waitFor(() => expect(input).toHaveFocus())
    await userEvent.setup().click(input)
    const listbox = await screen.findByRole('listbox')
    expect(visualPosition(listbox)).toEqual({
      left: 1050,
      top: 216,
      width: 400,
    })
  })

  it('sits right under the input outside any dialog', async () => {
    render(field())
    const input = screen.getByRole('combobox', { name: 'Provider' })
    mockLayout(input, null)
    await userEvent.setup().click(input)
    const listbox = await screen.findByRole('listbox')
    expect(visualPosition(listbox)).toEqual(underInput)
  })

  it('follows the input once the dialog open animation (scale 0.95) ends', async () => {
    render(
      <Dialog open title='Choose a provider'>
        {field()}
      </Dialog>
    )
    const dialog = screen.getByRole('dialog')
    const input = screen.getByRole('combobox', { name: 'Provider' })
    Object.defineProperty(dialog, 'offsetWidth', {
      value: 600,
      configurable: true,
    })
    mockLayout(input, dialog, {
      transform: `${centered} scale(0.95)`,
      dialogBox: { left: 310, top: 110 },
    })
    dialog.getBoundingClientRect = () =>
      ({ left: 310, top: 110, width: 600 * 0.95 }) as DOMRect
    input.getBoundingClientRect = rect({
      left: 345,
      top: 185,
      right: 725,
      bottom: 210,
      width: 380,
    })
    await waitFor(() => expect(input).toHaveFocus())
    await userEvent.setup().click(input)
    await screen.findByRole('listbox')

    dialog.style.transform = centered
    mockLayout(input, dialog)
    dialog.dispatchEvent(new Event('animationend', { bubbles: true }))
    dialog.dispatchEvent(new Event('transitionend', { bubbles: true }))

    await waitFor(() =>
      expect(visualPosition(screen.getByRole('listbox'))).toEqual(underInput)
    )
  })

  it('accounts for the border of a transformed container', async () => {
    render(
      <Dialog open title='Choose a provider'>
        {field()}
      </Dialog>
    )
    const dialog = screen.getByRole('dialog')
    const input = screen.getByRole('combobox', { name: 'Provider' })
    Object.defineProperty(dialog, 'clientLeft', {
      value: 3,
      configurable: true,
    })
    Object.defineProperty(dialog, 'clientTop', { value: 1, configurable: true })
    mockLayout(input, dialog, { transform: centered })
    await waitFor(() => expect(input).toHaveFocus())
    await userEvent.setup().click(input)
    const listbox = await screen.findByRole('listbox')
    expect(visualPosition(listbox)).toEqual(underInput)
  })

  it('matches the input visual size while the dialog is still scaled to 0.95', async () => {
    render(
      <Dialog open title='Choose a provider'>
        {field()}
      </Dialog>
    )
    const dialog = screen.getByRole('dialog')
    const input = screen.getByRole('combobox', { name: 'Provider' })
    Object.defineProperty(dialog, 'offsetWidth', {
      value: 600,
      configurable: true,
    })
    mockLayout(input, dialog, {
      transform: `${centered} scale(0.95)`,
      dialogBox: { left: 310, top: 110 },
    })
    dialog.getBoundingClientRect = () =>
      ({ left: 310, top: 110, width: 600 * 0.95 }) as DOMRect
    input.getBoundingClientRect = rect({
      left: 345,
      top: 185,
      right: 725,
      bottom: 210,
      width: 380,
    })
    await waitFor(() => expect(input).toHaveFocus())
    await userEvent.setup().click(input)
    const listbox = await screen.findByRole('listbox')
    const visual = visualPosition(listbox)
    expect(visual.width).toBeCloseTo(380, 0)
    expect(visual.left).toBeCloseTo(345, 0)
    expect(visual.top).toBeCloseTo(214, 0)
  })
})
