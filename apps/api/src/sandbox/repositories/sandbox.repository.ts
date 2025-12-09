/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import {
  Repository,
  DataSource,
  FindOptionsWhere,
  FindOneOptions,
  FindManyOptions,
  SelectQueryBuilder,
  DeleteResult,
} from 'typeorm'
import { Sandbox } from '../entities/sandbox.entity'
import { ConflictException, Injectable, NotFoundException } from '@nestjs/common'
import { InjectDataSource } from '@nestjs/typeorm'
import { EventEmitter2 } from '@nestjs/event-emitter'
import { SandboxEvents } from '../constants/sandbox-events.constants'
import { SandboxCreatedEvent } from '../events/sandbox-create.event'
import { SandboxStateUpdatedEvent } from '../events/sandbox-state-updated.event'
import { SandboxDesiredStateUpdatedEvent } from '../events/sandbox-desired-state-updated.event'
import { SandboxPublicStatusUpdatedEvent } from '../events/sandbox-public-status-updated.event'
import { SandboxOrganizationUpdatedEvent } from '../events/sandbox-organization-updated.event'

@Injectable()
export class SandboxRepository {
  private repository: Repository<Sandbox>

  constructor(
    @InjectDataSource() private dataSource: DataSource,
    private eventEmitter: EventEmitter2,
  ) {
    this.repository = this.dataSource.getRepository(Sandbox)
  }

  /**
   * See reference for {@link Repository.findOne}
   */
  async findOne(options: FindOneOptions<Sandbox>): Promise<Sandbox | null> {
    return this.repository.findOne(options)
  }

  /**
   * See reference for {@link Repository.findOneBy}
   */
  async findOneBy(where: FindOptionsWhere<Sandbox> | FindOptionsWhere<Sandbox>[]): Promise<Sandbox | null> {
    return this.repository.findOneBy(where)
  }

  /**
   * See reference for {@link Repository.findOneByOrFail}
   */
  async findOneByOrFail(where: FindOptionsWhere<Sandbox> | FindOptionsWhere<Sandbox>[]): Promise<Sandbox> {
    return this.repository.findOneByOrFail(where)
  }

  /**
   * See reference for {@link Repository.find}
   */
  async find(options?: FindManyOptions<Sandbox>): Promise<Sandbox[]> {
    return this.repository.find(options)
  }

  /**
   * See reference for {@link Repository.findAndCount}
   */
  async findAndCount(options?: FindManyOptions<Sandbox>): Promise<[Sandbox[], number]> {
    return this.repository.findAndCount(options)
  }

  /**
   * See reference for {@link Repository.count}
   */
  async count(options?: FindManyOptions<Sandbox>): Promise<number> {
    return this.repository.count(options)
  }

  /**
   * See reference for {@link Repository.createQueryBuilder}
   */
  createQueryBuilder(alias = 'sandbox'): SelectQueryBuilder<Sandbox> {
    return this.repository.createQueryBuilder(alias)
  }

  /**
   * See reference for {@link Repository.manager}
   */
  get manager() {
    return this.repository.manager
  }

  /**
   * Inserts a new sandbox into the database and emits a {@link SandboxCreatedEvent} event.
   *
   * Uses {@link Repository.insert} to insert the sandbox into the database.
   */
  async insert(sandbox: Sandbox): Promise<Sandbox> {
    const result = await this.repository.insert(sandbox)

    const insertedSandbox = await this.findOneBy({ id: result.identifiers[0].id })
    if (!insertedSandbox) {
      throw new NotFoundException('Sandbox not found after insert')
    }

    this.eventEmitter.emit(SandboxEvents.CREATED, new SandboxCreatedEvent(insertedSandbox))

    return insertedSandbox
  }

  /**
   * Partially updates a sandbox in the database and emits a corresponding event based on the changes.
   *
   * Uses {@link Repository.update} to update the sandbox in the database.
   */
  async update(sandbox: Sandbox, updateData: Partial<Sandbox>): Promise<Sandbox> {
    const oldState = sandbox.state
    const oldDesiredState = sandbox.desiredState
    const oldPublicStatus = sandbox.public
    const oldOrganizationId = sandbox.organizationId

    // Perform the update
    const result = await this.repository.update(sandbox.id, updateData)
    if (!result.affected) {
      throw new NotFoundException('Sandbox not found after update')
    }

    // Apply changes and emit events
    Object.assign(sandbox, updateData)
    this.emitUpdateEvents(sandbox, updateData, oldState, oldDesiredState, oldPublicStatus, oldOrganizationId)

    return sandbox
  }

  /**
   * Partially updates a sandbox in the database and emits a corresponding event based on the changes.
   *
   * Performs the update in a transaction with a pessimistic write lock to ensure consistency.
   *
   * @throws {ConflictException} if the sandbox was modified by another operation
   */
  async updateWhere(
    sandboxId: string,
    params: {
      updateData: Partial<Sandbox>
      whereCondition: FindOptionsWhere<Sandbox>
    },
  ): Promise<Sandbox> {
    const { updateData, whereCondition } = params

    return this.manager.transaction(async (entityManager) => {
      const whereClause = {
        ...whereCondition,
        id: sandboxId,
      }

      const sandbox = await entityManager.findOne(Sandbox, {
        where: whereClause,
        lock: { mode: 'pessimistic_write' },
        relations: [],
        loadEagerRelations: false,
      })

      if (!sandbox) {
        throw new ConflictException('Sandbox was modified by another operation, please try again')
      }

      // Store old values for event emission
      const oldState = sandbox.state
      const oldDesiredState = sandbox.desiredState
      const oldPublicStatus = sandbox.public
      const oldOrganizationId = sandbox.organizationId

      // Perform the update
      await entityManager.update(Sandbox, sandboxId, updateData)

      // Apply changes and emit events
      Object.assign(sandbox, updateData)
      this.emitUpdateEvents(sandbox, updateData, oldState, oldDesiredState, oldPublicStatus, oldOrganizationId)

      return sandbox
    })
  }

  /**
   * See reference for {@link Repository.delete}
   */
  async delete(criteria: FindOptionsWhere<Sandbox> | FindOptionsWhere<Sandbox>[]): Promise<DeleteResult> {
    return this.repository.delete(criteria)
  }

  /**
   * Emits events based on the changes made to a sandbox.
   */
  private emitUpdateEvents(
    updatedSandbox: Sandbox,
    updateData: Partial<Sandbox>,
    oldState: Sandbox['state'],
    oldDesiredState: Sandbox['desiredState'],
    oldPublicStatus: Sandbox['public'],
    oldOrganizationId: Sandbox['organizationId'],
  ): void {
    if (updateData.state !== undefined && oldState !== updateData.state) {
      this.eventEmitter.emit(
        SandboxEvents.STATE_UPDATED,
        new SandboxStateUpdatedEvent(updatedSandbox, oldState, updateData.state),
      )
    }

    if (updateData.desiredState !== undefined && oldDesiredState !== updateData.desiredState) {
      this.eventEmitter.emit(
        SandboxEvents.DESIRED_STATE_UPDATED,
        new SandboxDesiredStateUpdatedEvent(updatedSandbox, oldDesiredState, updateData.desiredState),
      )
    }

    if (updateData.public !== undefined && oldPublicStatus !== updateData.public) {
      this.eventEmitter.emit(
        SandboxEvents.PUBLIC_STATUS_UPDATED,
        new SandboxPublicStatusUpdatedEvent(updatedSandbox, oldPublicStatus, updateData.public),
      )
    }

    if (updateData.organizationId !== undefined && oldOrganizationId !== updateData.organizationId) {
      this.eventEmitter.emit(
        SandboxEvents.ORGANIZATION_UPDATED,
        new SandboxOrganizationUpdatedEvent(updatedSandbox, oldOrganizationId, updateData.organizationId),
      )
    }
  }
}
