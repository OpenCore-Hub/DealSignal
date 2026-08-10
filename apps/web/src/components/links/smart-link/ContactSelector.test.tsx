// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { ContactSelector } from "./ContactSelector";
import type { Contact } from "@/types";

const { createContactMock, getContactsMock } = vi.hoisted(() => ({
  createContactMock: vi.fn(),
  getContactsMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getContacts: getContactsMock,
    createContact: createContactMock,
  },
}));

const contact: Contact = {
  id: "c1",
  email: "alice@vc.com",
  name: "Alice",
  heatLevel: "warm",
  score: 80,
  scoreHistory: [],
  totalVisits: 3,
  totalDurationSeconds: 120,
  viewedDocuments: [],
};

async function setupI18n() {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["links", "linkShare", "common"],
    defaultNS: "links",
    resources: {
      en: {
        links: {
          creator: {
            contactLabel: "Recipients",
            contactHelper: "Codes go to these contacts",
            clearContact: "Clear",
            contactLoading: "Loading...",
            contactAddMore: "Add contact...",
            contactSearchPlaceholder: "Search contacts",
            contactEmpty: "No contacts yet.",
            contactNoResults: "No matches.",
            createContactFromSearch: 'Create "{{email}}" as contact',
          },
        },
        linkShare: {
          contactPicker: {
            addContact: "Add contact",
            addContactTitle: "Add contact",
            email: "Email",
            emailPlaceholder: "contact@example.com",
            name: "Name (optional)",
            namePlaceholder: "Contact name",
            create: "Create",
            creating: "Creating...",
          },
        },
        common: { cancel: "Cancel" },
      },
    },
    interpolation: { escapeValue: false },
  });
  return instance;
}

describe("ContactSelector", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getContactsMock.mockResolvedValue({ data: [contact] });
  });

  it("creates a contact inline and selects it without leaving the page", async () => {
    const onChange = vi.fn();
    createContactMock.mockResolvedValue({
      id: "c-new",
      email: "bob@fund.com",
      name: "Bob",
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-01T00:00:00Z",
    });
    const instance = await setupI18n();

    render(
      <I18nextProvider i18n={instance}>
        <ContactSelector
          workspaceSlug="acme"
          value={[]}
          onChange={onChange}
          contacts={[contact]}
        />
      </I18nextProvider>,
    );

    fireEvent.click(screen.getByTestId("contact-selector-trigger"));
    fireEvent.click(await screen.findByTestId("contact-add-new"));

    expect(await screen.findByTestId("contact-add-email")).toBeInTheDocument();
    fireEvent.change(screen.getByTestId("contact-add-email"), {
      target: { value: "bob@fund.com" },
    });
    fireEvent.change(screen.getByTestId("contact-add-name"), {
      target: { value: "Bob" },
    });
    fireEvent.click(screen.getByTestId("contact-add-submit"));

    await waitFor(() => {
      expect(createContactMock).toHaveBeenCalledWith(
        {
          email: "bob@fund.com",
          name: "Bob",
        },
        "acme",
      );
    });
    await waitFor(() => {
      expect(onChange).toHaveBeenCalledWith(["c-new"]);
    });
    await waitFor(() => {
      expect(screen.queryByTestId("contact-add-email")).not.toBeInTheDocument();
    });
  });

  it("closes dialog and selects when contacts are loaded via hook (no contacts prop)", async () => {
    const onChange = vi.fn();
    createContactMock.mockResolvedValue({
      id: "c-hook",
      email: "hook@fund.com",
      name: "Hook",
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-01T00:00:00Z",
    });
    const instance = await setupI18n();

    render(
      <I18nextProvider i18n={instance}>
        <ContactSelector workspaceSlug="acme" value={[]} onChange={onChange} />
      </I18nextProvider>,
    );

    await waitFor(() => {
      expect(getContactsMock).toHaveBeenCalledWith("acme");
    });
    await waitFor(() => {
      expect(screen.getByTestId("contact-selector-trigger")).not.toBeDisabled();
    });

    fireEvent.click(screen.getByTestId("contact-selector-trigger"));
    fireEvent.click(await screen.findByTestId("contact-add-new"));
    fireEvent.change(await screen.findByTestId("contact-add-email"), {
      target: { value: "hook@fund.com" },
    });
    fireEvent.click(screen.getByTestId("contact-add-submit"));

    await waitFor(() => {
      expect(onChange).toHaveBeenCalledWith(["c-hook"]);
    });
    await waitFor(() => {
      expect(screen.queryByTestId("contact-add-email")).not.toBeInTheDocument();
    });
  });
});
