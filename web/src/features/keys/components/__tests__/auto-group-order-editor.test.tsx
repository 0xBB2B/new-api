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
import { createEvent, fireEvent, render, within } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

const { useState } = await import('react')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { AutoGroupOrderEditor } = await import('../auto-group-order-editor')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        '{{count}} / {{max}} groups selected':
          '{{count}} / {{max}} groups selected',
        'Add Auto group': 'Add Auto group',
        'Add group': 'Add group',
        'Auto group order': 'Auto group order',
        'Drag {{group}} to reorder': 'Drag {{group}} to reorder',
        'Inherit global Auto order': 'Inherit global Auto order',
        'Maximum {{max}} groups selected': 'Maximum {{max}} groups selected',
        'Move {{group}} down': 'Move {{group}} down',
        'Move {{group}} up': 'Move {{group}} up',
        'No available groups in the global Auto order.':
          'No available groups in the global Auto order.',
        'No valid custom Auto groups remain. Add a group or restore global Auto.':
          'No valid custom Auto groups remain. Add a group or restore global Auto.',
        'No custom groups. Saving will inherit the complete global Auto order.':
          'No custom groups. Saving will inherit the complete global Auto order.',
        'Remove {{group}}': 'Remove {{group}}',
        'Restore global Auto': 'Restore global Auto',
        Ratio: 'Ratio',
        'Search...': 'Search...',
        'No group found.': 'No group found.',
        'Select a group': 'Select a group',
        'Using the complete global Auto order ({{count}} groups)':
          'Using the complete global Auto order ({{count}} groups)',
      },
    },
  },
})

const globalOptions = [
  { value: 'vip', label: 'VIP', desc: 'Priority access', ratio: 3 },
  { value: 'default', label: 'Default', desc: 'Standard access', ratio: 1 },
  { value: 'team', label: 'Team', desc: 'Shared access', ratio: 2 },
]

function Harness(props: { initialGroups?: string[] }) {
  const [groups, setGroups] = useState(
    props.initialGroups ?? ['default', 'vip']
  )
  const [mode, setMode] = useState<'inherit' | 'custom'>('custom')
  return (
    <I18nextProvider i18n={i18n}>
      <AutoGroupOrderEditor
        value={groups}
        mode={mode}
        options={[
          { value: 'auto', label: 'auto' },
          { value: 'default', label: 'default', ratio: 1 },
          { value: 'vip', label: 'vip', ratio: 2 },
          { value: 'team', label: 'team', ratio: 3 },
        ]}
        globalOptions={globalOptions}
        maxCount={2}
        onChange={(value) => {
          setGroups(value.groups)
          setMode(value.mode)
        }}
      />
      <output data-testid='order'>{groups.join(',')}</output>
      <output data-testid='mode'>{mode}</output>
    </I18nextProvider>
  )
}

function InheritanceHarness(props: { globalOptions?: typeof globalOptions }) {
  const [groups, setGroups] = useState<string[]>([])
  const [mode, setMode] = useState<'inherit' | 'custom'>('inherit')

  return (
    <I18nextProvider i18n={i18n}>
      <AutoGroupOrderEditor
        value={groups}
        mode={mode}
        options={[{ value: 'auto', label: 'auto' }, ...globalOptions]}
        globalOptions={props.globalOptions ?? globalOptions}
        maxCount={2}
        onChange={(value) => {
          setGroups(value.groups)
          setMode(value.mode)
        }}
      />
      <output data-testid='order'>{groups.join(',')}</output>
      <output data-testid='mode'>{mode}</output>
    </I18nextProvider>
  )
}

function CustomEmptyHarness() {
  const [groups, setGroups] = useState<string[]>([])
  const [mode, setMode] = useState<'inherit' | 'custom'>('custom')

  return (
    <I18nextProvider i18n={i18n}>
      <AutoGroupOrderEditor
        value={groups}
        mode={mode}
        options={[{ value: 'auto', label: 'auto' }, ...globalOptions]}
        globalOptions={globalOptions}
        maxCount={2}
        onChange={(value) => {
          setGroups(value.groups)
          setMode(value.mode)
        }}
      />
      <output data-testid='order'>{groups.join(',')}</output>
      <output data-testid='mode'>{mode}</output>
    </I18nextProvider>
  )
}

function findButton(container: HTMLElement, label: string): HTMLButtonElement {
  return within(container).getByRole('button', { name: label })
}

function getCommandItem(label: string): HTMLElement {
  const item = [
    ...document.querySelectorAll<HTMLElement>('[data-slot="command-item"]'),
  ].find((candidate) => candidate.textContent?.includes(label))
  if (!item) {
    throw new Error(`Expected command item containing "${label}"`)
  }
  return item
}

describe('Auto group order editor', () => {
  test('enforces the limit and exposes accessible reorder controls', () => {
    const { container } = render(<Harness />)

    const addButton = within(container).getByRole('combobox')
    expect(addButton).toBeDisabled()
    expect(container).toHaveTextContent('2 / 2 groups selected')
    expect(
      within(container).getByRole('group', { name: 'Auto group order' })
    ).toBeInTheDocument()
    expect(findButton(container, 'Drag default to reorder').type).toBe('button')

    fireEvent.click(findButton(container, 'Move default down'))
    expect(within(container).getByTestId('order')).toHaveTextContent(
      'vip,default'
    )

    fireEvent.keyDown(findButton(container, 'Drag vip to reorder'), {
      key: 'ArrowDown',
    })
    expect(within(container).getByTestId('order')).toHaveTextContent(
      'default,vip'
    )
  })

  test('adds and removes groups, then restores inheritance as an empty value', () => {
    const { container } = render(<Harness />)
    fireEvent.click(findButton(container, 'Remove vip'))

    expect(within(container).getByTestId('order')).toHaveTextContent('default')
    const addButton = within(container).getByRole('combobox')
    expect(addButton).toBeEnabled()

    fireEvent.click(addButton)
    fireEvent.click(getCommandItem('team'))
    expect(within(container).getByTestId('order')).toHaveTextContent(
      'default,team'
    )
    expect(addButton).toBeDisabled()

    fireEvent.click(
      within(container).getByRole('button', { name: 'Restore global Auto' })
    )

    expect(within(container).getByTestId('order')).toBeEmptyDOMElement()
    expect(within(container).getByTestId('mode')).toHaveTextContent('inherit')
    expect(container).toHaveTextContent(
      'Using the complete global Auto order (3 groups)'
    )

    const inheritedItems = container.querySelectorAll(
      '[data-slot="global-auto-order"] > li'
    )
    expect(
      [...inheritedItems].map(
        (item) =>
          item.querySelector('[data-slot="global-auto-order-name"]')
            ?.textContent
      )
    ).toEqual(['VIP', 'Default', 'Team'])
  })

  test('shows the complete inherited order with metadata beyond the custom limit', () => {
    const { container } = render(<InheritanceHarness />)

    expect(container).toHaveTextContent(
      'Using the complete global Auto order (3 groups)'
    )
    expect(container).not.toHaveTextContent('0 / 2 groups selected')

    const order = container.querySelector<HTMLOListElement>(
      '[data-slot="global-auto-order"]'
    )
    if (!order) {
      throw new Error('Expected inherited Auto group order')
    }
    expect(order).toHaveClass('overflow-y-auto', 'flex-wrap')

    const items = [...order.querySelectorAll('li')]
    expect(items.length).toBe(3)
    expect(
      order.querySelectorAll('[data-slot="global-auto-order-connector"]').length
    ).toBe(2)
    expect(
      items.map((item) => ({
        index: item.querySelector('[data-slot="global-auto-order-index"]')
          ?.textContent,
        name: item.querySelector('[data-slot="global-auto-order-name"]')
          ?.textContent,
        title: item
          .querySelector('[data-slot="global-auto-order-chip"]')
          ?.getAttribute('title'),
        description: item.querySelector(
          '[data-slot="global-auto-order-description"]'
        )?.textContent,
        ratio: item.querySelector('[data-slot="badge"]')?.textContent,
      }))
    ).toEqual([
      {
        index: '1',
        name: 'VIP',
        title: 'Priority access',
        description: 'Priority access',
        ratio: '3x',
      },
      {
        index: '2',
        name: 'Default',
        title: 'Standard access',
        description: 'Standard access',
        ratio: '1x',
      },
      {
        index: '3',
        name: 'Team',
        title: 'Shared access',
        description: 'Shared access',
        ratio: '2x',
      },
    ])

    for (const item of items) {
      const chip = item.querySelector('[data-slot="global-auto-order-chip"]')
      expect(chip).toBeInTheDocument()
      const description = item.querySelector(
        '[data-slot="global-auto-order-description"]'
      )
      expect(description).toHaveClass('sr-only')
    }

    expect(
      items[0]?.querySelector('[data-slot="global-auto-order-connector"]')
    ).toBe(null)
    for (const item of items.slice(1)) {
      const connector = item.querySelector(
        '[data-slot="global-auto-order-connector"]'
      )
      expect(connector).toHaveAttribute('aria-hidden', 'true')
    }

    expect(container.querySelector('[aria-label^="Drag "]')).toBe(null)
    expect(container.querySelector('[aria-label^="Move "]')).toBe(null)
    expect(container.querySelector('[aria-label^="Remove "]')).toBe(null)

    expect(
      within(container).getByRole('button', { name: 'Restore global Auto' })
    ).toBeDisabled()
  })

  test('shows an explicit empty state when the global Auto order has no groups', () => {
    const { container } = render(<InheritanceHarness globalOptions={[]} />)

    expect(container).toHaveTextContent(
      'Using the complete global Auto order (0 groups)'
    )
    expect(container).toHaveTextContent(
      'No available groups in the global Auto order.'
    )
    expect(container.querySelector('[data-slot="global-auto-order"]')).toBe(
      null
    )
  })

  test('keeps an empty custom order distinct from global inheritance', () => {
    const { container } = render(<CustomEmptyHarness />)

    expect(within(container).getByTestId('mode')).toHaveTextContent('custom')
    expect(container).toHaveTextContent(
      'No valid custom Auto groups remain. Add a group or restore global Auto.'
    )
    expect(container.querySelector('[data-slot="global-auto-order"]')).toBe(
      null
    )

    const restoreButton = within(container).getByRole('button', {
      name: 'Restore global Auto',
    })
    expect(restoreButton).toBeEnabled()
    fireEvent.click(restoreButton)

    expect(within(container).getByTestId('mode')).toHaveTextContent('inherit')
    expect(
      container.querySelector('[data-slot="global-auto-order"]')
    ).toBeInTheDocument()
  })

  test('adding a group from inheritance explicitly creates a custom order', () => {
    const { container } = render(<InheritanceHarness />)

    fireEvent.click(within(container).getByRole('combobox'))
    fireEvent.click(getCommandItem('VIP'))

    expect(within(container).getByTestId('mode')).toHaveTextContent('custom')
    expect(within(container).getByTestId('order')).toHaveTextContent('vip')
    expect(container.querySelector('[data-slot="global-auto-order"]')).toBe(
      null
    )
  })

  test('removing the last custom group does not silently enable inheritance', () => {
    const { container } = render(<Harness initialGroups={['default']} />)
    fireEvent.click(findButton(container, 'Remove default'))

    expect(within(container).getByTestId('order')).toBeEmptyDOMElement()
    expect(within(container).getByTestId('mode')).toHaveTextContent('custom')
    expect(container).toHaveTextContent(
      'No valid custom Auto groups remain. Add a group or restore global Auto.'
    )
  })
  test('hideInheritControls removes the selection count and the restore button', () => {
    const { container } = render(
      <I18nextProvider i18n={i18n}>
        <AutoGroupOrderEditor
          value={['default']}
          mode='custom'
          options={globalOptions}
          globalOptions={globalOptions}
          maxCount={2}
          hideInheritControls
          onChange={() => {}}
        />
      </I18nextProvider>
    )

    expect(
      within(container).queryByRole('button', { name: 'Restore global Auto' })
    ).toBeNull()
    expect(container).not.toHaveTextContent('1 / 2 groups selected')
    expect(findButton(container, 'Remove default')).toBeInTheDocument()
  })

  test('addLabel replaces the add placeholder', () => {
    const { container } = render(
      <I18nextProvider i18n={i18n}>
        <AutoGroupOrderEditor
          value={['default']}
          mode='custom'
          options={globalOptions}
          globalOptions={globalOptions}
          maxCount={2}
          addLabel='Add group'
          onChange={() => {}}
        />
      </I18nextProvider>
    )

    expect(
      within(container).getByRole('combobox', { name: 'Add group' })
    ).toBeInTheDocument()
    expect(container).not.toHaveTextContent('Add Auto group')
  })

  describe("variant='compact'", () => {
    const compactOptions = [
      { value: 'subscription', label: 'subscription' },
      { value: 'api', label: 'api' },
      { value: 'team', label: 'team' },
      { value: 'vip', label: 'vip' },
      { value: 'default', label: 'default' },
      { value: 'extra', label: 'extra' },
    ]

    function renderCompact(
      value: string[],
      extra: { maxCount?: number; 'aria-invalid'?: boolean } = {}
    ) {
      const onChange = vi.fn()
      const view = render(
        <I18nextProvider i18n={i18n}>
          <AutoGroupOrderEditor
            variant='compact'
            value={value}
            mode='custom'
            options={compactOptions}
            globalOptions={compactOptions}
            maxCount={extra.maxCount ?? 5}
            aria-invalid={extra['aria-invalid']}
            addLabel='Add group'
            onChange={onChange}
          />
        </I18nextProvider>
      )
      return { ...view, onChange }
    }

    function fireDragAt(
      type: 'dragOver' | 'drop',
      target: HTMLElement,
      clientX: number
    ) {
      const event = createEvent[type](target)
      Object.defineProperty(event, 'clientX', { value: clientX })
      fireEvent(target, event)
    }

    function getTag(container: HTMLElement, group: string): HTMLElement {
      const tag = within(container)
        .getByText(group)
        .closest<HTMLElement>('[draggable]')
      if (!tag) throw new Error(`Expected draggable tag for "${group}"`)
      return tag
    }

    test('shows numbered tags and a plus button for a non-empty value', () => {
      const { container } = renderCompact(['subscription', 'api'])

      const first = within(container).getByRole('listitem', {
        name: /subscription/,
      })
      const second = within(container).getByRole('listitem', { name: /api/ })
      expect(first).toHaveAccessibleName(/1/)
      expect(second).toHaveAccessibleName(/2/)
      expect(first).toHaveTextContent('1')
      expect(second).toHaveTextContent('2')
      expect(within(container).getAllByRole('combobox')).toHaveLength(1)
      expect(findButton(container, 'Remove subscription')).toBeInTheDocument()
      expect(findButton(container, 'Remove api')).toBeInTheDocument()
    })

    test('tags form a list and advertise the keyboard shortcut', () => {
      const { container } = renderCompact(['subscription', 'api'])

      expect(within(container).getByRole('list')).toHaveAttribute(
        'role',
        'list'
      )
      expect(within(container).getAllByRole('listitem')).toHaveLength(2)
      for (const tag of within(container).getAllByRole('listitem')) {
        expect(tag).toHaveAttribute(
          'aria-keyshortcuts',
          'Alt+ArrowLeft Alt+ArrowRight'
        )
      }
    })

    test('tags show a grab cursor only when reordering is possible', () => {
      const multi = renderCompact(['subscription', 'api'])
      expect(
        within(multi.container).getByRole('listitem', { name: /subscription/ })
      ).toHaveClass('cursor-grab')
      multi.unmount()

      const single = renderCompact(['subscription'])
      expect(
        within(single.container).getByRole('listitem', { name: /subscription/ })
      ).not.toHaveClass('cursor-grab')
    })

    test('focus stays on the moved tag after a keyboard move', () => {
      const onChange = vi.fn()
      const renderWith = (value: string[]) => (
        <I18nextProvider i18n={i18n}>
          <AutoGroupOrderEditor
            variant='compact'
            value={value}
            mode='custom'
            options={compactOptions}
            globalOptions={compactOptions}
            maxCount={5}
            addLabel='Add group'
            onChange={onChange}
          />
        </I18nextProvider>
      )
      const { container, rerender } = render(
        renderWith(['subscription', 'api'])
      )
      const tag = within(container).getByRole('listitem', {
        name: /subscription/,
      })
      tag.focus()
      fireEvent.keyDown(tag, { key: 'ArrowRight', altKey: true })
      expect(onChange).toHaveBeenCalledWith({
        groups: ['api', 'subscription'],
        mode: 'custom',
      })

      rerender(renderWith(['api', 'subscription']))

      expect(document.activeElement).toBe(
        within(container).getByRole('listitem', { name: /subscription/ })
      )
    })

    test.each([
      { name: 'last tag moved later', tag: 'team', key: 'ArrowRight' },
      { name: 'first tag moved earlier', tag: 'subscription', key: 'ArrowLeft' },
    ])(
      'a no-op keyboard move does not steal focus later ($name)',
      (scenario) => {
        const onChange = vi.fn()
        const renderWith = (value: string[]) => (
          <I18nextProvider i18n={i18n}>
            <AutoGroupOrderEditor
              variant='compact'
              value={value}
              mode='custom'
              options={compactOptions}
              globalOptions={compactOptions}
              maxCount={5}
              addLabel='Add group'
              onChange={onChange}
            />
          </I18nextProvider>
        )
        const { container, rerender } = render(
          renderWith(['subscription', 'api', 'team'])
        )
        const tag = within(container).getByRole('listitem', {
          name: new RegExp(scenario.tag),
        })
        tag.focus()
        fireEvent.keyDown(tag, { key: scenario.key, altKey: true })
        expect(onChange).not.toHaveBeenCalled()

        const plus = within(container).getByRole('combobox')
        plus.focus()
        rerender(renderWith(['subscription', 'api', 'team', 'vip']))

        expect(document.activeElement).toBe(
          within(container).getByRole('combobox')
        )
      }
    )

    test('dropping on the left half of a tag inserts before it', () => {
      const { container, onChange } = renderCompact([
        'subscription',
        'api',
        'default',
      ])
      const target = getTag(container, 'api')
      vi.spyOn(target, 'getBoundingClientRect').mockReturnValue({
        left: 0,
        width: 100,
        right: 100,
        top: 0,
        bottom: 20,
        height: 20,
        x: 0,
        y: 0,
        toJSON: () => ({}),
      })

      fireEvent.dragStart(getTag(container, 'default'))
      fireDragAt('dragOver', target, 10)
      fireDragAt('drop', target, 10)
      fireEvent.dragEnd(getTag(container, 'default'))

      expect(onChange).toHaveBeenCalledWith({
        groups: ['subscription', 'default', 'api'],
        mode: 'custom',
      })
    })

    test('dropping a tag back into its own position does not call onChange', () => {
      const { container, onChange } = renderCompact(['subscription', 'api'])
      const target = getTag(container, 'api')
      vi.spyOn(target, 'getBoundingClientRect').mockReturnValue({
        left: 0,
        width: 100,
        right: 100,
        top: 0,
        bottom: 20,
        height: 20,
        x: 0,
        y: 0,
        toJSON: () => ({}),
      })

      fireEvent.dragStart(getTag(container, 'subscription'))
      fireDragAt('dragOver', target, 10)
      fireDragAt('drop', target, 10)
      fireEvent.dragEnd(getTag(container, 'subscription'))

      expect(onChange).not.toHaveBeenCalled()
    })

    test('without addLabel the empty state uses the default Add group text', () => {
      const { container } = render(
        <I18nextProvider i18n={i18n}>
          <AutoGroupOrderEditor
            variant='compact'
            value={[]}
            mode='custom'
            options={compactOptions}
            globalOptions={compactOptions}
            maxCount={5}
            onChange={() => {}}
          />
        </I18nextProvider>
      )

      expect(container).toHaveTextContent('Add group')
      expect(
        within(container).getByRole('combobox', { name: 'Add group' })
      ).toBeInTheDocument()
    })

    test('shows only the add-group button for an empty value', () => {
      const { container } = renderCompact([])

      expect(within(container).getByText('Add group')).toBeInTheDocument()
      expect(container.querySelector('[aria-label^="Remove "]')).toBe(null)
    })

    test('dropping a tag after another reorders the groups', () => {
      const { container, onChange } = renderCompact(['subscription', 'api'])

      const target = getTag(container, 'api')
      vi.spyOn(target, 'getBoundingClientRect').mockReturnValue({
        left: 0,
        width: 100,
        right: 100,
        top: 0,
        bottom: 20,
        height: 20,
        x: 0,
        y: 0,
        toJSON: () => ({}),
      })

      fireEvent.dragStart(getTag(container, 'subscription'))
      fireDragAt('dragOver', target, 90)
      fireDragAt('drop', target, 90)
      fireEvent.dragEnd(getTag(container, 'subscription'))

      expect(onChange).toHaveBeenCalledWith({
        groups: ['api', 'subscription'],
        mode: 'custom',
      })
    })

    test('Alt+ArrowRight moves a tag one place later', () => {
      const { container, onChange } = renderCompact(['subscription', 'api'])

      fireEvent.keyDown(getTag(container, 'subscription'), {
        key: 'ArrowRight',
        altKey: true,
      })

      expect(onChange).toHaveBeenCalledWith({
        groups: ['api', 'subscription'],
        mode: 'custom',
      })
    })

    test('Alt+ArrowLeft on the first tag does not call onChange', () => {
      const { container, onChange } = renderCompact(['subscription', 'api'])

      fireEvent.keyDown(getTag(container, 'subscription'), {
        key: 'ArrowLeft',
        altKey: true,
      })

      expect(onChange).not.toHaveBeenCalled()
    })

    test('Alt+ArrowLeft moves a later tag one place earlier', () => {
      const { container, onChange } = renderCompact(['subscription', 'api'])

      fireEvent.keyDown(getTag(container, 'api'), {
        key: 'ArrowLeft',
        altKey: true,
      })

      expect(onChange).toHaveBeenCalledWith({
        groups: ['api', 'subscription'],
        mode: 'custom',
      })
    })

    test('a single group is not draggable', () => {
      const { container } = renderCompact(['subscription'])

      expect(within(container).getByText('subscription')).toBeInTheDocument()
      expect(container.querySelector('[draggable="true"]')).toBe(null)
    })

    test('the remove button drops that group', () => {
      const { container, onChange } = renderCompact(['subscription', 'api'])

      fireEvent.click(findButton(container, 'Remove subscription'))

      expect(onChange).toHaveBeenCalledWith({ groups: ['api'], mode: 'custom' })
    })

    test('the plus button appends the chosen group to the end', () => {
      const { container, onChange } = renderCompact(['subscription', 'api'])

      fireEvent.click(within(container).getByRole('combobox'))
      fireEvent.click(getCommandItem('team'))

      expect(onChange).toHaveBeenCalledWith({
        groups: ['subscription', 'api', 'team'],
        mode: 'custom',
      })
    })

    test('the plus button is disabled at the limit', () => {
      const { container } = renderCompact(
        ['subscription', 'api', 'team', 'vip', 'default'],
        { maxCount: 5 }
      )

      expect(within(container).getByRole('combobox')).toBeDisabled()
    })

    test('aria-invalid reaches the root element', () => {
      const { container } = renderCompact(['subscription'], {
        'aria-invalid': true,
      })

      expect(container.querySelector('[aria-invalid="true"]')).not.toBe(null)
    })
  })
})
