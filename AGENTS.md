### File and Folder Structure

**Applicability:**
These principles apply to all code, not just React components. While React’s multi-file nature (e.g., components, hooks, styles) makes it a clear example, the same logic applies to endpoints, utilities, and everything else, in any programming language.

**Files:**
Refactor a file into a folder with well-named modules if it:

- Exceeds ~500 lines, **or**
- Handles multiple responsibilities (violates SRP).

**Folders:**

- Represent one feature/component.
- **Co-locate** all related items (logic, styles, tests, utilities) (see note below for details).
- If there is a single primary export, name the folder after the original file (e.g., `Button/` for `Button.tsx`).
- If there are multiple exports, use dash-case (e.g., `auth-helpers/`) and describe the category in the folder’s `AGENTS.md`.

**Fractal Structure:**
Every folder, regardless of depth, follows the same rules. A sub-component’s folder (e.g., `Button/Icon/`) should co-locate its own logic, styles, tests, and utilities just like the parent component.

**Index files**:
index.ts files should only function as barrel files, they should not contain implementation.

**Co-location principles:**

- First and foremost, co-locate by feature, not by "type" (e.g., "css files", "hooks"). A folder defining a React component or a backend service should contain everything specific to it: sub-components, helpers, styles, tests, etc.
- When a file/module is reused across features, move it up the hierarchy to the lowest common ancestor folder.
- There are a few exceptions, for instance:
  - Namespaces (e.g., `integrations/` for an array of integrations).
  - Technical requirements (e.g., generated data models in a `models/` folder).
- Consider the _Law of Demeter_ for imports: avoid deep relative paths (e.g., `../../OtherComponent/someHelper`). If needed, refactor to flatten the hierarchy.

### AGENTS.md Files

**Goal:** The fractal structure aims for cognitive encapsulation — any sub-tree should be understandable in isolation, without loading the rest of the app into context. AGENTS.md files serve this goal the way code comments do: they surface what naming and structure alone don't convey.

**Location:** Every source folder and subfolder should include an `AGENTS.md`.

**Content:**

- **Purpose**: 1–2 sentences (e.g., "Manages user authentication").
- **Notes**: Document gotchas, unconventional patterns, known tech debt, or context not obvious from the code or naming.
- **Key Files**: List critical files/modules and their roles (skip obvious details).
- **Relationships**: Note dependencies (e.g., "Uses `../utils` for helpers").

**Rules:**

- **Brevity**: Prioritize succinctness for LLM token efficiency. **Omit details derivable from conventions, naming, or folder structure.**
- **Prioritize the Notes section** for non-obvious context (e.g., gotchas, tech debt).
- **Updates**: Required when:
  - Adding/removing files.
  - Changing responsibilities.
  - Creating subfolders (also update parent’s `AGENTS.md`).

---

**Example:**

```markdown
# `/src/auth` AGENTS.md

**Purpose**: User authentication and session management.
**Notes**:

- Uses a custom session store due to legacy constraints (see #123).
- Avoid modifying `middleware.ts` without updating `login.ts`.

**Key Files**:

- `login.ts`: Login logic.
- `middleware.ts`: Auth validation.
  **Relationships**: `../utils/crypto` for hashing.
```

---

### Testing Philosophy

Prioritize _semantic coverage_ (testing behavior) over line coverage. **Focus on critical paths and refactoring safety.** Tests should enable safe refactoring; skip trivial paths (e.g., simple getters/setters) that add no value. The pattern is to create a file with the same name + `.test.ts(x)` or sanme name + `_test.go` next to the file being tested.

For integration tests covering the interaction between multiple files, use a `{concept}.test.ts` at the appropriate folder level. If that file exceeds ~500 lines, apply the same file→folder rule: refactor to a `{concept}.test/` folder with named sub-files inside.
