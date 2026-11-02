import type { SearchResult } from '@/types'
import { Store } from './store-base'

interface SearchState {
  searchCounts: number
  searchResults: SearchResult[]
  currentPage: number
  pageSize: number
  totalPages: number
}

class SearchStore extends Store<SearchState> {
  constructor() {
    super({ searchCounts: 0, searchResults: [], currentPage: 1, pageSize: 20, totalPages: 0 })
  }

  updateSearchResults(results: SearchResult[], totalNum: number) {
    const totalPages = Math.ceil(totalNum / this.state.pageSize)
    this.setState({ searchResults: [...results], searchCounts: totalNum, totalPages })
  }

  setPage(page: number) {
    this.setState({ currentPage: page })
  }

  clearSearchResults() {
    this.setState({ searchCounts: 0, searchResults: [], currentPage: 1, totalPages: 0 })
  }
}

export const searchStore = new SearchStore()
