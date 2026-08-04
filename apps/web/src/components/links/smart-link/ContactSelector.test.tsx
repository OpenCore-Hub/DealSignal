// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { ContactSelector } from "./ContactSelector";
import type { Contact } from "@/types";

const { createContactMock } = vi.hoisted(() => ({
  createContactMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getContacts: vi.fn(),
    createContact: createContactMock,
  },
}));

const contact: Contact = {
  id: "c1",
  email: "alice@vc.com",
  name: "Alice",
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
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
      expect(createContactMock).toHaveBeenCalledWith({
        email: "bob@fund.com",
        name: "Bob",
      });
    });
    await waitFor(() => {
      expect(onChange).toHaveBeenCalledWith(["c-new"]);
    });
  });
});
